package tests

import (
	"context"
	"sync"
	"testing"
)

func createOffer(t *testing.T, c *client, qty int, overrides map[string]any) map[string]any {
	t.Helper()
	body := map[string]any{"category": "drinking_water", "title": "Water depot", "quantity_available": qty,
		"unit": "litres", "pickup_or_delivery_mode": "pickup",
		"location": map[string]any{"mode": "protected_exact", "area_label": "East depot",
			"exact": map[string]any{"address": "Depot Road 3"}}}
	for k, v := range overrides {
		body[k] = v
	}
	return c.must(201, "POST", "/api/v1/offers", body)
}

func TestOverAllocationPreventedUnderConcurrency(t *testing.T) {
	e := newEnv(t)
	coord := e.login("coord", "coordinator")
	offer := createOffer(t, coord, 10, nil)
	req := verifiedRequest(t, coord, coord, nil)

	// Sequential over-allocation is rejected with the remaining quantity.
	out := coord.must(409, "POST", "/api/v1/assignments", map[string]any{
		"request_id": req["id"], "offer_id": offer["id"], "quantity_assigned": 11})
	if errCode(out) != "over_allocation" {
		t.Fatalf("expected over_allocation: %v", out)
	}

	// 25 concurrent allocations of 1 unit: exactly 10 may succeed.
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var res map[string]any
			s := coord.do("POST", "/api/v1/assignments", map[string]any{
				"request_id": req["id"], "offer_id": offer["id"], "quantity_assigned": 1}, &res)
			mu.Lock()
			statuses[s]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if statuses[201] != 10 || statuses[409] != 15 {
		t.Fatalf("expected 10 created / 15 conflicts, got %v", statuses)
	}
	o := coord.must(200, "GET", "/api/v1/offers/"+offer["id"].(string), nil)
	if o["remaining_quantity"].(float64) != 0 || o["status"] != "fully_allocated" {
		t.Fatalf("offer state wrong: remaining=%v status=%v", o["remaining_quantity"], o["status"])
	}
	// The database constraint is the last line of defence.
	_, err := e.pool.Exec(context.Background(), `UPDATE offers SET assigned_quantity = quantity_available + 1 WHERE id = $1`, offer["id"])
	if err == nil {
		t.Fatal("database must reject over-allocation")
	}
}

func TestAllocationReleasedAndRequestFlow(t *testing.T) {
	e := newEnv(t)
	coord := e.login("coord", "coordinator")
	owner := e.login("owner", "requester")
	vol := e.login("vol", "volunteer")
	other := e.login("vol2", "volunteer")
	offer := createOffer(t, coord, 50, nil)
	req := verifiedRequest(t, owner, coord, map[string]any{
		"location": map[string]any{"mode": "protected_exact", "area_label": "Old town",
			"exact": map[string]any{"address": "Secret Street 42"}},
		"contact": map[string]any{"method": "phone", "visibility": "assigned_responders", "details": "+49 111"},
	})
	rid := req["id"].(string)

	// Unverified requests cannot be assigned.
	unverified := owner.must(201, "POST", "/api/v1/requests", baseRequest(nil))
	out := coord.must(409, "POST", "/api/v1/assignments", map[string]any{"request_id": unverified["id"], "team_label": "Team A"})
	if errCode(out) != "request_not_assignable" {
		t.Fatalf("expected request_not_assignable: %v", out)
	}
	// Volunteers cannot create assignments.
	vol.must(403, "POST", "/api/v1/assignments", map[string]any{"request_id": rid, "team_label": "Me"})
	// Grants require an individual volunteer.
	coord.must(422, "POST", "/api/v1/assignments", map[string]any{"request_id": rid, "team_label": "Team",
		"destination_location_access_granted": true})

	a := coord.must(201, "POST", "/api/v1/assignments", map[string]any{
		"request_id": rid, "offer_id": offer["id"], "quantity_assigned": 40, "volunteer_user_id": vol.userID(),
		"instructions": "Pick up at depot gate B.", "destination_location_access_granted": true,
		"pickup_location_access_granted": true})
	aid := a["id"].(string)
	if a["status"] != "proposed" {
		t.Fatalf("assignment should start proposed: %v", a)
	}
	r := coord.must(200, "GET", "/api/v1/requests/"+rid, nil)
	if r["status"] != "assigned" {
		t.Fatalf("request should be assigned after assignment creation, got %v", r["status"])
	}

	// Other volunteers see nothing; the assigned volunteer sees a limited view.
	other.must(404, "GET", "/api/v1/assignments/"+aid, nil)
	other.must(404, "GET", "/api/v1/requests/"+rid, nil)
	vr := vol.must(200, "GET", "/api/v1/requests/"+rid, nil)
	if vr["viewer_relation"] != "assigned_volunteer" || vr["created_by"] != nil || vr["review_status"] != nil {
		t.Fatalf("volunteer view leaks data: %v", vr)
	}
	if list := vol.must(200, "GET", "/api/v1/assignments", nil); len(list["items"].([]any)) != 1 {
		t.Fatalf("volunteer should see exactly their assignment: %v", list)
	}

	// Before accepting, no protected access.
	vol.must(403, "POST", "/api/v1/assignments/"+aid+"/protected", nil)
	// Volunteer cannot start request work before accepting.
	vol.must(403, "POST", "/api/v1/requests/"+rid+"/status", map[string]any{"status": "in_progress"})
	vol.must(200, "POST", "/api/v1/assignments/"+aid+"/status", map[string]any{"status": "accepted"})
	p := vol.must(200, "POST", "/api/v1/assignments/"+aid+"/protected", nil)
	if p["destination_location"].(map[string]any)["address"] != "Secret Street 42" || p["pickup_location"] == nil {
		t.Fatalf("granted locations not revealed: %v", p)
	}
	if p["destination_contact"] != nil {
		t.Fatal("contact was not granted and must not be revealed")
	}
	// Volunteer cannot cancel or grant themselves access.
	vol.must(403, "POST", "/api/v1/assignments/"+aid+"/status", map[string]any{"status": "cancelled", "reason": "nope"})
	vol.must(403, "PATCH", "/api/v1/assignments/"+aid, map[string]any{"protected_contact_access_granted": true})
	coord.must(200, "PATCH", "/api/v1/assignments/"+aid, map[string]any{"protected_contact_access_granted": true})
	p = vol.must(200, "POST", "/api/v1/assignments/"+aid+"/protected", nil)
	if p["destination_contact"].(map[string]any)["details"] != "+49 111" {
		t.Fatalf("contact grant not effective: %v", p)
	}

	// Starting the assignment moves the request to in_progress.
	vol.must(200, "POST", "/api/v1/assignments/"+aid+"/status", map[string]any{"status": "in_progress", "eta_text": "15 min"})
	if r := coord.must(200, "GET", "/api/v1/requests/"+rid, nil); r["status"] != "in_progress" {
		t.Fatalf("request should be in_progress, got %v", r["status"])
	}
	// Delivery never resolves the request automatically.
	d := vol.must(200, "POST", "/api/v1/assignments/"+aid+"/status", map[string]any{"status": "delivered",
		"handover_status": "handed_over", "handover_notes": "Handed to shelter desk."})
	if d["completion_evidence_type"] != "volunteer_confirmation" {
		t.Fatalf("default evidence wrong: %v", d)
	}
	if r := coord.must(200, "GET", "/api/v1/requests/"+rid, nil); r["status"] != "in_progress" {
		t.Fatalf("request must not be auto-resolved, got %v", r["status"])
	}
	// After delivery protected access ends.
	vol.must(403, "POST", "/api/v1/assignments/"+aid+"/protected", nil)
	dash := coord.must(200, "GET", "/api/v1/dashboard", nil)
	if dash["requests"].(map[string]any)["awaiting_confirmation"].(float64) != 1 {
		t.Fatalf("dashboard should show awaiting confirmation: %v", dash["requests"])
	}
	coord.must(422, "POST", "/api/v1/requests/"+rid+"/status", map[string]any{"status": "resolved"})
	coord.must(200, "POST", "/api/v1/requests/"+rid+"/status", map[string]any{"status": "resolved", "resolution_summary": "40 l delivered."})

	// A second request: cancellation releases allocations.
	req2 := verifiedRequest(t, owner, coord, nil)
	a2 := coord.must(201, "POST", "/api/v1/assignments", map[string]any{
		"request_id": req2["id"], "offer_id": offer["id"], "quantity_assigned": 10})
	o := coord.must(200, "GET", "/api/v1/offers/"+offer["id"].(string), nil)
	if o["remaining_quantity"].(float64) != 0 {
		t.Fatalf("remaining should be 0, got %v", o["remaining_quantity"])
	}
	// Closing a request with active assignments needs explicit confirmation.
	out = coord.must(409, "POST", "/api/v1/requests/"+req2["id"].(string)+"/status", map[string]any{"status": "cancelled", "reason": "Not needed"})
	if errCode(out) != "active_assignments" {
		t.Fatalf("expected active_assignments: %v", out)
	}
	coord.must(200, "POST", "/api/v1/requests/"+req2["id"].(string)+"/status",
		map[string]any{"status": "cancelled", "reason": "Not needed", "cancel_active_assignments": true})
	a2v := coord.must(200, "GET", "/api/v1/assignments/"+a2["id"].(string), nil)
	if a2v["status"] != "cancelled" {
		t.Fatalf("assignment should be cancelled: %v", a2v["status"])
	}
	o = coord.must(200, "GET", "/api/v1/offers/"+offer["id"].(string), nil)
	if o["remaining_quantity"].(float64) != 10 || o["status"] != "partially_allocated" {
		t.Fatalf("allocation not released: remaining=%v status=%v", o["remaining_quantity"], o["status"])
	}
}

func TestOfferPermissions(t *testing.T) {
	e := newEnv(t)
	vol := e.login("vol", "volunteer")
	vol2 := e.login("vol2", "volunteer")
	coord := e.login("coord", "coordinator")
	requester := e.login("req", "requester")

	requester.must(403, "POST", "/api/v1/offers", map[string]any{"category": "food", "title": "Soup", "quantity_available": 5})
	o := createOffer(t, vol, 20, map[string]any{"location": map[string]any{"mode": "area_only", "area_label": "Center"}})
	id := o["id"].(string)
	if o["verification_level"] != "self_reported" {
		t.Fatalf("volunteer offer should be self_reported: %v", o["verification_level"])
	}
	vol2.must(404, "GET", "/api/v1/offers/"+id, nil)
	if l := vol2.must(200, "GET", "/api/v1/offers", nil); len(l["items"].([]any)) != 0 {
		t.Fatal("volunteers must not browse other offers")
	}
	coord.must(200, "GET", "/api/v1/offers/"+id, nil)
	// Derived statuses cannot be set manually.
	coord.must(409, "POST", "/api/v1/offers/"+id+"/status", map[string]any{"status": "fully_allocated"})
	vol.must(403, "PATCH", "/api/v1/offers/"+id, map[string]any{"verification_level": "field_confirmed"})
	vol.must(200, "POST", "/api/v1/offers/"+id+"/status", map[string]any{"status": "paused"})
	vol.must(200, "POST", "/api/v1/offers/"+id+"/status", map[string]any{"status": "available"})
	// Quantity can never drop below the allocated amount.
	req := verifiedRequest(t, coord, coord, nil)
	coord.must(201, "POST", "/api/v1/assignments", map[string]any{"request_id": req["id"], "offer_id": id, "quantity_assigned": 15})
	coord.must(422, "PATCH", "/api/v1/offers/"+id, map[string]any{"quantity_available": 10})
	vol.must(403, "PATCH", "/api/v1/offers/"+id, map[string]any{"title": "Changed after allocation"})
}
