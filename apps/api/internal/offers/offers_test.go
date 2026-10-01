package offers

import "testing"

func TestDerivedStatus(t *testing.T) {
	cases := []struct {
		cur             Status
		avail, assigned int
		want            Status
	}{
		{StatusAvailable, 10, 0, StatusAvailable},
		{StatusAvailable, 10, 3, StatusPartiallyAllocated},
		{StatusPartiallyAllocated, 10, 10, StatusFullyAllocated},
		{StatusFullyAllocated, 10, 4, StatusPartiallyAllocated},
		{StatusFullyAllocated, 10, 0, StatusAvailable},
		{StatusPaused, 10, 10, StatusPaused},
		{StatusDraft, 10, 0, StatusDraft},
		{StatusCancelled, 10, 0, StatusCancelled},
	}
	for _, c := range cases {
		if got := DerivedStatus(c.cur, c.avail, c.assigned); got != c.want {
			t.Errorf("DerivedStatus(%s,%d,%d)=%s want %s", c.cur, c.avail, c.assigned, got, c.want)
		}
	}
}

func TestDerivedStatusesNotManuallySettable(t *testing.T) {
	for _, st := range AllStatuses {
		o := &Record{Status: Status(st)}
		for _, allowed := range append(coordinatorAllowedStatuses(o), ownerAllowedStatuses(o)...) {
			if allowed == StatusPartiallyAllocated || allowed == StatusFullyAllocated {
				t.Errorf("%s must not be settable manually (from %s)", allowed, st)
			}
		}
	}
}

func TestOwnerCannotCancelAllocatedOffer(t *testing.T) {
	o := &Record{Status: StatusPartiallyAllocated, AssignedQuantity: 3}
	for _, s := range ownerAllowedStatuses(o) {
		if s == StatusCancelled {
			t.Fatal("owner must not cancel an offer with allocations")
		}
	}
}
