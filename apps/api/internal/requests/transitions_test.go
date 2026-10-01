package requests

import "testing"

func TestOnlyCoordinatorsVerifyAssignOrClose(t *testing.T) {
	coordOnly := []Status{StatusVerified, StatusAssigned, StatusResolved, StatusCancelled, StatusDuplicate, StatusExpired}
	for from := range coordinatorTransitions {
		for _, to := range coordOnly {
			if CanTransition(from, to, ActorOwner) {
				t.Errorf("owner must not move %s -> %s", from, to)
			}
			if CanTransition(from, to, ActorAssignedVolunteer) {
				t.Errorf("volunteer must not move %s -> %s", from, to)
			}
		}
	}
}

func TestInProgressByCoordinatorOrAssignedVolunteer(t *testing.T) {
	if !CanTransition(StatusAssigned, StatusInProgress, ActorAssignedVolunteer) {
		t.Fatal("assigned volunteer should be able to start work")
	}
	if !CanTransition(StatusAssigned, StatusInProgress, ActorCoordinator) {
		t.Fatal("coordinator should be able to set in_progress")
	}
	if CanTransition(StatusAssigned, StatusInProgress, ActorOwner) {
		t.Fatal("requester must not set in_progress")
	}
	if CanTransition(StatusSubmitted, StatusInProgress, ActorAssignedVolunteer) {
		t.Fatal("volunteer cannot start an unassigned request")
	}
}

func TestOwnerCanOnlySubmitDraft(t *testing.T) {
	if !CanTransition(StatusDraft, StatusSubmitted, ActorOwner) {
		t.Fatal("owner should submit drafts")
	}
	for _, from := range AllStatuses {
		for _, to := range AllStatuses {
			if from == StatusDraft && to == StatusSubmitted {
				continue
			}
			if CanTransition(from, to, ActorOwner) {
				t.Errorf("owner must not move %s -> %s", from, to)
			}
		}
	}
}

func TestClosedStatusesAreTerminal(t *testing.T) {
	for _, s := range AllStatuses {
		if !s.IsClosed() {
			continue
		}
		for _, k := range []ActorKind{ActorOwner, ActorAssignedVolunteer, ActorCoordinator} {
			if len(NextStatuses(s, k)) != 0 {
				t.Errorf("closed status %s must have no direct transitions (use reopen)", s)
			}
		}
		if !CanReopen(s, ActorCoordinator) {
			t.Errorf("coordinator should be able to reopen %s", s)
		}
		if CanReopen(s, ActorOwner) || CanReopen(s, ActorAssignedVolunteer) {
			t.Errorf("only coordinators may reopen %s", s)
		}
	}
	if CanReopen(StatusSubmitted, ActorCoordinator) {
		t.Fatal("open requests cannot be reopened")
	}
}

func TestEveryStatusReachable(t *testing.T) {
	reached := map[Status]bool{StatusDraft: true}
	queue := []Status{StatusDraft}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, n := range NextStatuses(s, ActorCoordinator) {
			if !reached[n] {
				reached[n] = true
				queue = append(queue, n)
			}
		}
	}
	for _, s := range AllStatuses {
		if !reached[s] {
			t.Errorf("status %s unreachable", s)
		}
	}
}

func TestDuplicateOnlyBeforeAssignment(t *testing.T) {
	for _, from := range []Status{StatusAssigned, StatusInProgress, StatusPartiallyResolved} {
		if CanTransition(from, StatusDuplicate, ActorCoordinator) {
			t.Errorf("%s -> duplicate should not be allowed", from)
		}
	}
}
