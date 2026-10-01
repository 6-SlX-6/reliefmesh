package requests

import "slices"

// Status is a request lifecycle status.
type Status string

const (
	StatusDraft             Status = "draft"
	StatusSubmitted         Status = "submitted"
	StatusUnderReview       Status = "under_review"
	StatusVerified          Status = "verified"
	StatusAssigned          Status = "assigned"
	StatusInProgress        Status = "in_progress"
	StatusPartiallyResolved Status = "partially_resolved"
	StatusResolved          Status = "resolved"
	StatusCancelled         Status = "cancelled"
	StatusExpired           Status = "expired"
	StatusDuplicate         Status = "duplicate"
)

// AllStatuses lists statuses in lifecycle order.
var AllStatuses = []Status{StatusDraft, StatusSubmitted, StatusUnderReview, StatusVerified, StatusAssigned,
	StatusInProgress, StatusPartiallyResolved, StatusResolved, StatusCancelled, StatusExpired, StatusDuplicate}

// StatusStrings returns all statuses as strings.
func StatusStrings() []string {
	out := make([]string, len(AllStatuses))
	for i, s := range AllStatuses {
		out[i] = string(s)
	}
	return out
}

// IsClosed reports whether s is terminal. Closed requests can only be
// reopened by a coordinator, with a reason.
func (s Status) IsClosed() bool {
	return s == StatusResolved || s == StatusCancelled || s == StatusExpired || s == StatusDuplicate
}

// IsOpen reports whether s is neither a draft nor closed.
func (s Status) IsOpen() bool { return s != StatusDraft && !s.IsClosed() }

// ActorKind describes the relationship of the actor to the request for the
// purpose of status transitions.
type ActorKind int

const (
	ActorOwner ActorKind = iota
	ActorAssignedVolunteer
	ActorCoordinator
)

// coordinatorTransitions is the full transition graph available to
// coordinators (and organization managers). Reopening closed requests is a
// separate, reason-bearing action and therefore not listed here.
var coordinatorTransitions = map[Status][]Status{
	StatusDraft:             {StatusSubmitted, StatusCancelled},
	StatusSubmitted:         {StatusUnderReview, StatusVerified, StatusCancelled, StatusDuplicate, StatusExpired},
	StatusUnderReview:       {StatusVerified, StatusCancelled, StatusDuplicate, StatusExpired},
	StatusVerified:          {StatusAssigned, StatusUnderReview, StatusCancelled, StatusDuplicate, StatusExpired},
	StatusAssigned:          {StatusInProgress, StatusPartiallyResolved, StatusResolved, StatusVerified, StatusCancelled, StatusExpired},
	StatusInProgress:        {StatusPartiallyResolved, StatusResolved, StatusAssigned, StatusCancelled, StatusExpired},
	StatusPartiallyResolved: {StatusInProgress, StatusAssigned, StatusResolved, StatusCancelled, StatusExpired},
}

// ownerTransitions: requesters may only submit their own drafts.
var ownerTransitions = map[Status][]Status{
	StatusDraft: {StatusSubmitted},
}

// volunteerTransitions: assigned volunteers may only mark work as started.
var volunteerTransitions = map[Status][]Status{
	StatusAssigned:          {StatusInProgress},
	StatusPartiallyResolved: {StatusInProgress},
}

// NextStatuses returns the statuses an actor of the given kind may move a
// request to from the current status.
func NextStatuses(from Status, kind ActorKind) []Status {
	var m map[Status][]Status
	switch kind {
	case ActorCoordinator:
		m = coordinatorTransitions
	case ActorAssignedVolunteer:
		m = volunteerTransitions
	default:
		m = ownerTransitions
	}
	out := slices.Clone(m[from])
	if out == nil {
		out = []Status{}
	}
	return out
}

// CanTransition reports whether the transition is allowed for the actor.
func CanTransition(from, to Status, kind ActorKind) bool {
	return slices.Contains(NextStatuses(from, kind), to)
}

// CanReopen reports whether a request in status s can be reopened by the
// actor. Only coordinators may reopen, and only closed requests.
func CanReopen(s Status, kind ActorKind) bool {
	return kind == ActorCoordinator && s.IsClosed()
}

// ReopenTarget is the status a reopened request returns to.
const ReopenTarget = StatusUnderReview
