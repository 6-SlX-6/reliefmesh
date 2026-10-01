package tests

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRequestVisibilityByRole(t *testing.T) {
	e := newEnv(t)
	reqA := e.login("req-a", "requester")
	reqB := e.login("req-b", "requester")
	vol := e.login("vol", "volunteer")
	coord := e.login("coord", "coordinator")
	admin := e.login("root", "admin")

	created := reqA.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	id := created["id"].(string)
	if !strings.HasPrefix(created["reference"].(string), "RM-") {
		t.Fatalf("unexpected reference %v", created["reference"])
	}
	if created["viewer_relation"] != "owner" || created["status"] != "submitted" {
		t.Fatalf("unexpected view %v", created)
	}

	// Other requesters, unassigned volunteers and admins cannot see it (404, not 403).
	reqB.must(404, "GET", "/api/v1/requests/"+id, nil)
	vol.must(404, "GET", "/api/v1/requests/"+id, nil)
	admin.must(404, "GET", "/api/v1/requests/"+id, nil)
	admin.must(403, "GET", "/api/v1/requests", nil)
	for _, c := range []*client{reqB, vol} {
		list := c.must(200, "GET", "/api/v1/requests", nil)
		if n := len(list["items"].([]any)); n != 0 {
			t.Fatalf("%s sees %d foreign requests", c.user["username"], n)
		}
	}
	// Coordinator sees everything including the creator, owner sees no internals.
	cv := coord.must(200, "GET", "/api/v1/requests/"+id, nil)
	if cv["created_by"] == nil || cv["viewer_relation"] != "coordinator" {
		t.Fatalf("coordinator view incomplete: %v", cv)
	}
	ov := reqA.must(200, "GET", "/api/v1/requests/"+id, nil)
	if ov["created_by"] != nil || ov["tags"] != nil || ov["assigned_user"] != nil {
		t.Fatalf("owner view leaks internals: %v", ov)
	}
	// Protected fields never appear in normal views.
	for _, v := range []map[string]any{cv, ov} {
		for _, k := range []string{"contact_details", "exact_location", "contact_details_sealed"} {
			if _, ok := v[k]; ok {
				t.Fatalf("view exposes %s", k)
			}
		}
	}
}

func TestStatusTransitionRules(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	coord := e.login("coord", "coordinator")
	vol := e.login("vol", "volunteer")

	r := owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	id := r["id"].(string)
	path := "/api/v1/requests/" + id + "/status"

	// Requesters cannot verify, resolve or cancel.
	for _, st := range []string{"verified", "cancelled", "duplicate", "expired", "under_review"} {
		out := owner.must(403, "POST", path, map[string]any{"status": st, "reason": "because"})
		if errCode(out) != "forbidden" {
			t.Fatalf("owner %s: %v", st, out)
		}
	}
	// Invalid transition is rejected with allowed statuses.
	out := coord.must(409, "POST", path, map[string]any{"status": "resolved", "resolution_summary": "done"})
	if errCode(out) != "invalid_transition" {
		t.Fatalf("expected invalid_transition: %v", out)
	}
	coord.must(200, "POST", path, map[string]any{"status": "under_review"})
	// Owner can no longer edit once review started.
	out = owner.must(403, "PATCH", "/api/v1/requests/"+id, map[string]any{"title": "Changed title"})
	if errCode(out) != "edit_locked" {
		t.Fatalf("expected edit_locked: %v", out)
	}
	// Owner can still add a shared update note.
	owner.must(201, "POST", "/api/v1/requests/"+id+"/notes", map[string]any{"body": "We are now 25 people."})
	owner.must(403, "POST", "/api/v1/requests/"+id+"/notes", map[string]any{"body": "x", "visibility": "internal"})

	coord.must(200, "POST", path, map[string]any{"status": "verified"})
	// Assigned status requires an assignment.
	out = coord.must(409, "POST", path, map[string]any{"status": "assigned"})
	if errCode(out) != "no_assignment" {
		t.Fatalf("expected no_assignment: %v", out)
	}
	// Cancel requires a reason.
	coord.must(422, "POST", path, map[string]any{"status": "cancelled"})
	cancelled := coord.must(200, "POST", path, map[string]any{"status": "cancelled", "reason": "Need covered by town hall"})
	if cancelled["closed_at"] == nil {
		t.Fatal("closed_at not set")
	}
	// Closed requests: no transitions, reopen only by coordinator with reason.
	coord.must(409, "POST", path, map[string]any{"status": "verified"})
	owner.must(403, "POST", "/api/v1/requests/"+id+"/reopen", map[string]any{"reason": "please"})
	coord.must(422, "POST", "/api/v1/requests/"+id+"/reopen", map[string]any{"reason": ""})
	reopened := coord.must(200, "POST", "/api/v1/requests/"+id+"/reopen", map[string]any{"reason": "Town hall supply fell through"})
	if reopened["status"] != "under_review" || reopened["closed_at"] != nil {
		t.Fatalf("reopen failed: %v", reopened)
	}

	// Every transition is in the immutable timeline, with reasons.
	tl := coord.must(200, "GET", "/api/v1/requests/"+id+"/events", nil)
	var transitions []string
	for _, ev := range tl["items"].([]any) {
		m := ev.(map[string]any)
		if m["action"] == "request.status_changed" || m["action"] == "request.reopened" {
			transitions = append(transitions, m["from_status"].(string)+">"+m["to_status"].(string))
		}
	}
	want := []string{"submitted>under_review", "under_review>verified", "verified>cancelled", "cancelled>under_review"}
	if strings.Join(transitions, ",") != strings.Join(want, ",") {
		t.Fatalf("timeline transitions %v, want %v", transitions, want)
	}
	// Owner timeline hides actor names; the volunteer cannot read it at all.
	otl := owner.must(200, "GET", "/api/v1/requests/"+id+"/events", nil)
	for _, ev := range otl["items"].([]any) {
		actor := ev.(map[string]any)["actor"].(map[string]any)
		if actor["display_name"] != nil || actor["user_id"] != nil {
			t.Fatalf("owner timeline exposes staff identity: %v", actor)
		}
	}
	vol.must(404, "GET", "/api/v1/requests/"+id+"/events", nil)
}

func TestDuplicateRequiresCanonical(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	coord := e.login("coord", "coordinator")
	a := owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	b := owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	path := "/api/v1/requests/" + b["id"].(string) + "/status"
	coord.must(422, "POST", path, map[string]any{"status": "duplicate"})
	coord.must(422, "POST", path, map[string]any{"status": "duplicate", "duplicate_of_reference": "RM-0000-9999999"})
	coord.must(422, "POST", path, map[string]any{"status": "duplicate", "duplicate_of_reference": b["reference"]})
	dup := coord.must(200, "POST", path, map[string]any{"status": "duplicate", "duplicate_of_reference": a["reference"]})
	if dup["duplicate_of"].(map[string]any)["reference"] != a["reference"] {
		t.Fatalf("canonical not recorded: %v", dup)
	}
	// A duplicate cannot be used as canonical.
	c := owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	coord.must(422, "POST", "/api/v1/requests/"+c["id"].(string)+"/status",
		map[string]any{"status": "duplicate", "duplicate_of_reference": b["reference"]})
}

func TestCriticalUrgencyRequiresEmergencyNoticeAcknowledgement(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	out := owner.must(422, "POST", "/api/v1/requests", baseRequest(map[string]any{"urgency": "critical"}))
	if _, ok := out["error"].(map[string]any)["fields"].(map[string]any)["emergency_notice_acknowledged"]; !ok {
		t.Fatalf("expected acknowledgement error: %v", out)
	}
	owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{"urgency": "critical", "emergency_notice_acknowledged": true}))
	// Upgrading to critical later also requires the acknowledgement.
	r := owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	owner.must(422, "PATCH", "/api/v1/requests/"+r["id"].(string), map[string]any{"urgency": "critical"})
	owner.must(200, "PATCH", "/api/v1/requests/"+r["id"].(string), map[string]any{"urgency": "critical", "emergency_notice_acknowledged": true})
}

func TestMedicinePickupIsLogisticsOnly(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	out := owner.must(422, "POST", "/api/v1/requests", baseRequest(map[string]any{
		"category": "medicine_pickup", "title": "Insulin for diabetes", "description": "Insulin 100 IU/ml prescription"}))
	if _, ok := out["error"].(map[string]any)["fields"].(map[string]any)["description"]; !ok {
		t.Fatalf("medical free text must be rejected: %v", out)
	}
	r := owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{
		"category": "medicine_pickup", "title": "Insulin for diabetes", "description": "",
		"requires_formal_authorization": true}))
	if r["title"] != "Medicine pickup (logistics only)" {
		t.Fatalf("title must be replaced, got %q", r["title"])
	}
	if r["sensitive_data_flag"] != true || r["requires_formal_authorization"] != true {
		t.Fatalf("flags not set: %v", r)
	}
	var stored string
	_ = e.pool.QueryRow(context.Background(), `SELECT title FROM aid_requests WHERE id = $1`, r["id"]).Scan(&stored)
	if strings.Contains(strings.ToLower(stored), "insulin") {
		t.Fatal("medical title persisted")
	}
	// Sensitive details are not part of list responses for coordinators.
	coord := e.login("coord", "coordinator")
	sens := owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{"sensitive_data_flag": true,
		"description": "private detail", "accessibility_notes": "private access note"}))
	list := coord.must(200, "GET", "/api/v1/requests", nil)
	for _, it := range list["items"].([]any) {
		m := it.(map[string]any)
		if m["id"] == sens["id"] && (m["description"] != "" || m["details_redacted"] != true) {
			t.Fatalf("sensitive description leaked in list: %v", m)
		}
	}
	detail := coord.must(200, "GET", "/api/v1/requests/"+sens["id"].(string), nil)
	if detail["description"] != "private detail" {
		t.Fatal("coordinator detail should include the description")
	}
}

func TestProtectedDataEncryptedAndRevealAudited(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	coord := e.login("coord", "coordinator")
	r := owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{
		"location": map[string]any{"mode": "protected_exact", "area_label": "Old town",
			"exact": map[string]any{"lat": 52.520008, "lon": 13.404954, "address": "Secret Street 42"}},
		"contact": map[string]any{"method": "phone", "visibility": "coordinators_only", "details": "+49 30 1234567"},
	}))
	id := r["id"].(string)
	loc := r["location"].(map[string]any)
	if loc["approx_lat"].(float64) != 52.52 || loc["has_exact"] != true {
		t.Fatalf("approximation wrong: %v", loc)
	}
	var contact, exact []byte
	_ = e.pool.QueryRow(context.Background(), `SELECT contact_details_sealed, exact_location_sealed FROM aid_requests WHERE id = $1`, id).Scan(&contact, &exact)
	if bytes.Contains(contact, []byte("1234567")) || bytes.Contains(exact, []byte("Secret Street")) || len(contact) == 0 {
		t.Fatal("protected values must be encrypted at rest")
	}
	revealed := coord.must(200, "POST", "/api/v1/requests/"+id+"/protected", nil)
	if revealed["contact"].(map[string]any)["details"] != "+49 30 1234567" {
		t.Fatalf("reveal failed: %v", revealed)
	}
	if revealed["exact_location"].(map[string]any)["address"] != "Secret Street 42" {
		t.Fatalf("reveal failed: %v", revealed)
	}
	// The requester sees in their timeline that protected data was viewed.
	tl := owner.must(200, "GET", "/api/v1/requests/"+id+"/events", nil)
	var seen bool
	for _, ev := range tl["items"].([]any) {
		m := ev.(map[string]any)
		if m["action"] == "request.protected_viewed" {
			seen = true
			if m["actor"].(map[string]any)["label"] != "Coordinator" {
				t.Fatalf("unexpected actor %v", m["actor"])
			}
		}
	}
	if !seen {
		t.Fatal("protected view not audited in requester timeline")
	}
	// Volunteers without assignment cannot reveal anything.
	vol := e.login("vol", "volunteer")
	vol.must(404, "POST", "/api/v1/requests/"+id+"/protected", nil)
}

func TestOptimisticConcurrency(t *testing.T) {
	e := newEnv(t)
	coord := e.login("coord", "coordinator")
	r := coord.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	id := r["id"].(string)
	v := int(r["version"].(float64))
	coord.must(200, "PATCH", "/api/v1/requests/"+id, map[string]any{"version": v, "urgency": "high"})
	out := coord.must(409, "PATCH", "/api/v1/requests/"+id, map[string]any{"version": v, "urgency": "low"})
	if errCode(out) != "version_conflict" {
		t.Fatalf("expected version_conflict: %v", out)
	}
}

func TestClientIDMakesCreateIdempotent(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	other := e.login("other", "requester")
	cid := "6b0c6d8e-3a2f-4c1e-9b7a-1f2e3d4c5b6a"
	first := owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{"client_id": cid}))
	second := owner.must(200, "POST", "/api/v1/requests", baseRequest(map[string]any{"client_id": cid}))
	if first["id"] != second["id"] {
		t.Fatal("retry created a duplicate")
	}
	other.must(409, "POST", "/api/v1/requests", baseRequest(map[string]any{"client_id": cid}))
}
