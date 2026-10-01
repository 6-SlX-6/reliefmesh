package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAuditLogIsAppendOnlyAndVerifiable(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	coord := e.login("coord", "coordinator")
	verifiedRequest(t, owner, coord, nil)
	ctx := context.Background()
	for _, sql := range []string{
		`UPDATE audit_events SET action = 'tampered'`,
		`DELETE FROM audit_events`,
		`TRUNCATE audit_events`,
	} {
		if _, err := e.pool.Exec(ctx, sql); err == nil {
			t.Fatalf("%q must be rejected", sql)
		}
	}
	admin := e.login("root", "admin")
	res := admin.must(200, "GET", "/api/v1/admin/audit/verify", nil)
	if res["valid"] != true || res["events_checked"].(float64) < 4 {
		t.Fatalf("chain should verify: %v", res)
	}
	// Tampering below the application (bypassing the trigger as superuser
	// would) is detected by the hash chain.
	if _, err := e.pool.Exec(ctx, `ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete`); err == nil {
		_, _ = e.pool.Exec(ctx, `UPDATE audit_events SET to_status = 'resolved' WHERE action = 'request.status_changed'`)
		_, _ = e.pool.Exec(ctx, `ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete`)
		res = admin.must(200, "GET", "/api/v1/admin/audit/verify", nil)
		if res["valid"] != false {
			t.Fatal("tampering not detected")
		}
	}
	// Audit events never contain protected values.
	list := admin.must(200, "GET", "/api/v1/admin/audit?limit=500", nil)
	for _, ev := range list["items"].([]any) {
		m := ev.(map[string]any)
		if strings.Contains(strings.ToLower(fmt.Sprint(m["reason"])+strings.Join(keys(m["metadata"]), ",")), "password") {
			t.Fatalf("audit event leaks secrets: %v", m)
		}
	}
}

func keys(v any) []string {
	m, _ := v.(map[string]any)
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestSyncPushIdempotencyAndConflicts(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	coord := e.login("coord", "coordinator")
	clientID := uuid.NewString()
	op1 := uuid.NewString()
	createdAt := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	ops := []map[string]any{
		{"op_id": op1, "type": "request.create", "client_created_at": createdAt,
			"payload": baseRequest(map[string]any{"client_id": clientID})},
		{"op_id": uuid.NewString(), "type": "request.note", "entity_client_id": clientID,
			"payload": map[string]any{"client_id": uuid.NewString(), "body": "Written while offline"}},
		{"op_id": uuid.NewString(), "type": "request.status", "entity_client_id": clientID,
			"payload": map[string]any{"status": "verified"}},
		{"op_id": uuid.NewString(), "type": "request.create", "payload": map[string]any{"bogus": true}},
	}
	res := owner.must(200, "POST", "/api/v1/sync/push", map[string]any{"operations": ops})
	results := res["results"].([]any)
	want := []string{"applied", "applied", "rejected", "rejected"}
	for i, r := range results {
		if st := r.(map[string]any)["status"]; st != want[i] {
			t.Fatalf("op %d: status %v, want %s (%v)", i, st, want[i], r)
		}
	}
	reqID := results[0].(map[string]any)["entity_id"].(string)
	if !strings.HasPrefix(results[0].(map[string]any)["reference"].(string), "RM-") {
		t.Fatal("applied create should return the server reference")
	}

	// Replaying the same batch is idempotent.
	res = owner.must(200, "POST", "/api/v1/sync/push", map[string]any{"operations": ops[:2]})
	for _, r := range res["results"].([]any) {
		if st := r.(map[string]any)["status"]; st != "duplicate" {
			t.Fatalf("replay should be duplicate, got %v", st)
		}
	}
	var count int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM aid_requests`).Scan(&count)
	if count != 1 {
		t.Fatalf("replay created duplicates: %d requests", count)
	}
	notes := owner.must(200, "GET", "/api/v1/requests/"+reqID+"/notes", nil)
	if len(notes["items"].([]any)) != 1 {
		t.Fatalf("expected one note: %v", notes)
	}
	// Offline creation is marked in the audit trail.
	tl := coord.must(200, "GET", "/api/v1/requests/"+reqID+"/events", nil)
	if tl["items"].([]any)[0].(map[string]any)["metadata"].(map[string]any)["created_offline"] != true {
		t.Fatal("offline creation not recorded")
	}

	// Conflicting status changes from two offline coordinators.
	coord2 := e.login("coord2", "coordinator")
	coord.must(200, "POST", "/api/v1/requests/"+reqID+"/status", map[string]any{"status": "cancelled", "reason": "Handled elsewhere"})
	res = coord2.must(200, "POST", "/api/v1/sync/push", map[string]any{"operations": []map[string]any{
		{"op_id": uuid.NewString(), "type": "request.status", "entity_id": reqID, "payload": map[string]any{"status": "verified", "version": 1}},
	}})
	r := res["results"].([]any)[0].(map[string]any)
	if r["status"] != "conflict" || r["entity"].(map[string]any)["status"] != "cancelled" {
		t.Fatalf("expected conflict with current entity: %v", r)
	}
	// Stale field updates conflict as well.
	res = coord2.must(200, "POST", "/api/v1/sync/push", map[string]any{"operations": []map[string]any{
		{"op_id": uuid.NewString(), "type": "request.update", "entity_id": reqID, "payload": map[string]any{"version": 1, "urgency": "high"}},
	}})
	if st := res["results"].([]any)[0].(map[string]any)["status"]; st != "conflict" {
		t.Fatalf("stale update should conflict, got %v", st)
	}
	// Another user cannot replay someone else's operation id.
	res = coord.must(200, "POST", "/api/v1/sync/push", map[string]any{"operations": ops[:1]})
	if st := res["results"].([]any)[0].(map[string]any)["status"]; st != "rejected" {
		t.Fatalf("foreign op id must be rejected, got %v", st)
	}
}

func TestSyncPullScopesData(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	other := e.login("other", "requester")
	owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	full := owner.must(200, "GET", "/api/v1/sync/pull", nil)
	if full["full"] != true || len(full["requests"].([]any)) != 1 || full["settings"] == nil {
		t.Fatalf("unexpected pull: %v", full)
	}
	if len(other.must(200, "GET", "/api/v1/sync/pull", nil)["requests"].([]any)) != 0 {
		t.Fatal("pull leaks other users' requests")
	}
	since := full["server_time"].(string)
	inc := owner.must(200, "GET", "/api/v1/sync/pull?since="+since, nil)
	if inc["full"] != false {
		t.Fatal("incremental pull expected")
	}
	admin := e.login("root", "admin")
	ap := admin.must(200, "GET", "/api/v1/sync/pull", nil)
	if len(ap["requests"].([]any)) != 0 || len(ap["offers"].([]any)) != 0 {
		t.Fatal("admins must not receive operational data")
	}
}

func TestExportContainsNoPersonalData(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	coord := e.login("coord", "coordinator")
	manager := e.login("manager", "organization_manager")
	owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{
		"description": "Mrs Example in flat 3", "title": "Water for Mrs Example",
		"contact":  map[string]any{"method": "phone", "details": "+49 999 000"},
		"location": map[string]any{"mode": "area_only", "area_label": "=HYPERLINK(\"http://evil\")"}}))
	owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{"sensitive_data_flag": true,
		"location": map[string]any{"mode": "area_only", "area_label": "Private Lane"}}))
	coord.must(403, "GET", "/api/v1/exports/requests.csv", nil)
	var csv string
	if s := manager.do("GET", "/api/v1/exports/requests.csv", nil, &csv); s != 200 {
		t.Fatalf("export failed: %d", s)
	}
	for _, leak := range []string{"Mrs Example", "+49 999", "flat 3", "Private Lane"} {
		if strings.Contains(csv, leak) {
			t.Fatalf("export leaks %q:\n%s", leak, csv)
		}
	}
	if !strings.Contains(csv, `'=HYPERLINK`) {
		t.Fatalf("formula injection not neutralized:\n%s", csv)
	}
	sum := manager.must(200, "GET", "/api/v1/exports/summary.json", nil)
	if sum["requests_total"].(float64) != 2 {
		t.Fatalf("unexpected summary: %v", sum)
	}
}

func TestRetentionRedactsClosedRequests(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	coord := e.login("coord", "coordinator")
	admin := e.login("root", "admin")
	r := owner.must(201, "POST", "/api/v1/requests", baseRequest(map[string]any{
		"contact": map[string]any{"method": "phone", "details": "+49 1"}}))
	id := r["id"].(string)
	coord.must(200, "POST", "/api/v1/requests/"+id+"/status", map[string]any{"status": "cancelled", "reason": "Test case"})
	_, err := e.pool.Exec(context.Background(), `UPDATE aid_requests SET closed_at = now() - interval '400 days' WHERE id = $1`, id)
	if err != nil {
		t.Fatal(err)
	}
	pv := admin.must(200, "GET", "/api/v1/admin/retention", nil)
	if pv["requests"].(float64) != 1 {
		t.Fatalf("preview: %v", pv)
	}
	admin.must(422, "POST", "/api/v1/admin/retention/apply", map[string]any{"confirm": false})
	admin.must(200, "POST", "/api/v1/admin/retention/apply", map[string]any{"confirm": true})
	v := coord.must(200, "GET", "/api/v1/requests/"+id, nil)
	if v["title"] != "[redacted]" || v["description"] != "" || v["has_contact_details"] != false || v["redacted"] != true {
		t.Fatalf("not redacted: %v", v)
	}
	if v["category"] != "drinking_water" {
		t.Fatal("statistical fields must be kept")
	}
}

func TestAdminDeleteByReference(t *testing.T) {
	e := newEnv(t)
	owner := e.login("owner", "requester")
	admin := e.login("root", "admin")
	coord := e.login("coord", "coordinator")
	r := owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	coord.must(403, "POST", "/api/v1/admin/records/delete", map[string]any{"entity_type": "request", "reference": r["reference"], "reason": "Data subject request"})
	admin.must(422, "POST", "/api/v1/admin/records/delete", map[string]any{"entity_type": "request", "reference": r["reference"], "reason": ""})
	admin.must(204, "POST", "/api/v1/admin/records/delete", map[string]any{"entity_type": "request", "reference": r["reference"], "reason": "Data subject request"})
	owner.must(404, "GET", "/api/v1/requests/"+r["id"].(string), nil)
	coord.must(404, "GET", "/api/v1/requests/"+r["id"].(string), nil)
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'request.deleted'`).Scan(&n)
	if n != 1 {
		t.Fatal("deletion must be audited")
	}
}

func TestVolunteerAvailability(t *testing.T) {
	e := newEnv(t)
	vol := e.login("vol", "volunteer")
	vol2 := e.login("vol2", "volunteer")
	coord := e.login("coord", "coordinator")
	manager := e.login("manager", "organization_manager")
	vol.must(204, "PATCH", "/api/v1/me/availability", map[string]any{"availability": "limited", "availability_note": "Until 18:00"})
	vol.must(403, "PATCH", "/api/v1/volunteers/"+vol2.userID()+"/availability", map[string]any{"availability": "unavailable"})
	coord.must(403, "PATCH", "/api/v1/volunteers/"+vol2.userID()+"/availability", map[string]any{"availability": "unavailable"})
	manager.must(204, "PATCH", "/api/v1/volunteers/"+vol2.userID()+"/availability", map[string]any{"availability": "unavailable"})
	list := coord.must(200, "GET", "/api/v1/volunteers", nil)
	if len(list["items"].([]any)) != 2 {
		t.Fatalf("expected 2 volunteers: %v", list)
	}
	vol.must(403, "GET", "/api/v1/volunteers", nil)
}

func TestDemoSeedAndDashboard(t *testing.T) {
	e := newEnv(t)
	admin := e.login("root", "admin")
	info := admin.must(200, "GET", "/api/v1/admin/demo", nil)
	if info["enabled"] != true || len(info["scenarios"].([]any)) != 3 {
		t.Fatalf("unexpected demo info: %v", info)
	}
	admin.must(204, "POST", "/api/v1/admin/demo/seed", map[string]any{"scenario": "flood"})
	admin.must(409, "POST", "/api/v1/admin/demo/seed", map[string]any{"scenario": "flood"})
	dash := admin.must(200, "GET", "/api/v1/dashboard", nil)
	if dash["queues"] != nil {
		t.Fatal("admin dashboard must not include queues with request titles")
	}
	if dash["requests"].(map[string]any)["open"].(float64) < 5 {
		t.Fatalf("unexpected counts: %v", dash["requests"])
	}
	res := admin.must(200, "GET", "/api/v1/admin/audit/verify", nil)
	if res["valid"] != true {
		t.Fatalf("demo seeding must keep the audit chain valid: %v", res)
	}
}
