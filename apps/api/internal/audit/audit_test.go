package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashIsDeterministicAcrossMetadataRoundTrip(t *testing.T) {
	id := uuid.New()
	s := &Stored{ID: 7, OccurredAt: time.Unix(1_700_000_000, 123456000).UTC(), EntityID: &id,
		Action: "request.status_changed", EntityType: "aid_request", FromStatus: "submitted", ToStatus: "verified",
		Visibility: Shared, ActorRoles: []string{"coordinator"},
		Metadata: map[string]any{"quantity": 5, "b": "x", "a": true}}
	h1, err := computeHash(genesis, s)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate reading back from jsonb: numbers become float64, key order differs.
	s.Metadata = map[string]any{"a": true, "quantity": float64(5), "b": "x"}
	h2, _ := computeHash(genesis, s)
	if string(h1) != string(h2) {
		t.Fatal("hash must be stable across metadata decoding")
	}
	s.ToStatus = "resolved"
	h3, _ := computeHash(genesis, s)
	if string(h1) == string(h3) {
		t.Fatal("hash must change when content changes")
	}
	h4, _ := computeHash(h1, s)
	if string(h3) == string(h4) {
		t.Fatal("hash must depend on previous hash")
	}
}
