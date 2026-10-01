package assignments

import (
	"slices"
	"testing"
)

func TestVolunteerCannotCancel(t *testing.T) {
	for from := range transitions {
		if slices.Contains(NextStatuses(from, false), StatusCancelled) {
			t.Errorf("volunteer must not cancel from %s", from)
		}
		if !slices.Contains(NextStatuses(from, true), StatusCancelled) {
			t.Errorf("coordinator should cancel from %s", from)
		}
	}
}

func TestTerminalStatusesHaveNoTransitions(t *testing.T) {
	for _, s := range AllStatuses {
		st := Status(s)
		if st.IsTerminal() && len(NextStatuses(st, true)) != 0 {
			t.Errorf("terminal status %s has transitions", s)
		}
		if st.IsTerminal() == st.IsActive() {
			t.Errorf("status %s must be either terminal or active", s)
		}
	}
}

func TestReleasingStatuses(t *testing.T) {
	for _, s := range []Status{StatusDeclined, StatusUnableToComplete, StatusCancelled} {
		if !s.releasesAllocation() {
			t.Errorf("%s should release allocation", s)
		}
	}
	for _, s := range []Status{StatusDelivered, StatusPartiallyDelivered, StatusAccepted} {
		if s.releasesAllocation() {
			t.Errorf("%s must keep allocation", s)
		}
	}
}

func TestEvidenceTypesNeverRequireBiometrics(t *testing.T) {
	for _, e := range EvidenceTypes {
		for _, banned := range []string{"photo", "image", "biometric", "id_document", "signature"} {
			if slices.Contains([]string{e}, banned) {
				t.Errorf("evidence type %s not allowed", e)
			}
		}
	}
	if !slices.Contains(EvidenceTypes, "no_evidence") {
		t.Fatal("no_evidence must be available")
	}
}
