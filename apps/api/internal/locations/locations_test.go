package locations

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/config"
	"github.com/6-slx-6/reliefmesh/apps/api/internal/validation"
)

func f(v float64) *float64 { return &v }

func TestRound(t *testing.T) {
	cases := []struct {
		in   float64
		dec  int
		want float64
	}{
		{52.520008, 2, 52.52},
		{13.404954, 2, 13.40},
		{13.405954, 2, 13.41},
		{-33.86882, 1, -33.9},
		{52.520008, 0, 53},
	}
	for _, c := range cases {
		if got := Round(c.in, c.dec); got != c.want {
			t.Errorf("Round(%v,%d)=%v want %v", c.in, c.dec, got, c.want)
		}
	}
}

func TestNormalizeApproximateRoundsAndCaps(t *testing.T) {
	errs := validation.Errors{}
	n := Normalize(Input{Mode: "approximate", Lat: f(52.520008), Lon: f(13.404954)}, 2, errs)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if *n.ApproxLat != 52.52 || *n.ApproxLon != 13.40 {
		t.Fatalf("not rounded: %v %v", *n.ApproxLat, *n.ApproxLon)
	}
	// Precision finer than the maximum is capped.
	n = Normalize(Input{Mode: "approximate", Lat: f(52.520008), Lon: f(13.404954)}, 6, errs)
	if *n.ApproxLat != 52.52 || *n.Decimals != MaxApproxDecimals {
		t.Fatalf("precision not capped: %v %v", *n.ApproxLat, *n.Decimals)
	}
}

func TestNormalizeNoneDiscardsData(t *testing.T) {
	errs := validation.Errors{}
	n := Normalize(Input{Mode: "none", AreaLabel: "Somewhere", Lat: f(1), Lon: f(2),
		Exact: &ExactLocation{Address: "Main St 1"}}, 2, errs)
	if n.AreaLabel != "" || n.ApproxLat != nil || n.Exact != nil || n.ExactBytes != nil {
		t.Fatalf("mode none must not keep data: %+v", n)
	}
}

func TestNormalizeAreaOnlyRequiresArea(t *testing.T) {
	errs := validation.Errors{}
	n := Normalize(Input{Mode: "area_only", Lat: f(1), Lon: f(2)}, 2, errs)
	if !errs.Has("location.area_label") {
		t.Fatal("expected area_label error")
	}
	if n.ApproxLat != nil {
		t.Fatal("area_only must not keep coordinates")
	}
}

func TestNormalizeProtectedExactDerivesApproximation(t *testing.T) {
	errs := validation.Errors{}
	n := Normalize(Input{Mode: "protected_exact", AreaLabel: "North district",
		Exact: &ExactLocation{Lat: f(48.137154), Lon: f(11.576124), Address: "Example Street 5"}}, 2, errs)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if *n.ApproxLat != 48.14 || *n.ApproxLon != 11.58 {
		t.Fatalf("approximation wrong: %v %v", *n.ApproxLat, *n.ApproxLon)
	}
	if !bytes.Contains(n.ExactBytes, []byte("Example Street 5")) {
		t.Fatal("exact payload missing address")
	}
}

func TestNormalizeRejectsInvalid(t *testing.T) {
	for _, in := range []Input{
		{Mode: "bogus"},
		{Mode: "approximate", Lat: f(91), Lon: f(0)},
		{Mode: "protected_exact"},
		{Mode: "protected_exact", Exact: &ExactLocation{}},
	} {
		errs := validation.Errors{}
		Normalize(in, 2, errs)
		if len(errs) == 0 {
			t.Errorf("expected error for %+v", in)
		}
	}
}

func testKey(id string, b byte) config.DataKey {
	return config.DataKey{ID: id, Key: bytes.Repeat([]byte{b}, 32)}
}

func TestSealerRoundTripAndBinding(t *testing.T) {
	s, err := NewSealer([]config.DataKey{testKey("k1", 1)})
	if err != nil {
		t.Fatal(err)
	}
	ct, err := s.Seal([]byte("secret"), AAD("aid_request", "id1", "contact"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("secret")) {
		t.Fatal("ciphertext contains plaintext")
	}
	pt, err := s.Open(ct, AAD("aid_request", "id1", "contact"))
	if err != nil || string(pt) != "secret" {
		t.Fatalf("round trip failed: %v %q", err, pt)
	}
	if _, err := s.Open(ct, AAD("aid_request", "id2", "contact")); err == nil {
		t.Fatal("ciphertext must be bound to its row")
	}
	if _, err := s.Open(ct, AAD("aid_request", "id1", "exact_location")); err == nil {
		t.Fatal("ciphertext must be bound to its field")
	}
	ct[len(ct)-1] ^= 0xFF
	if _, err := s.Open(ct, AAD("aid_request", "id1", "contact")); err == nil {
		t.Fatal("tampered ciphertext must fail")
	}
}

func TestSealerKeyRotation(t *testing.T) {
	old, _ := NewSealer([]config.DataKey{testKey("k1", 1)})
	ct, _ := old.Seal([]byte("v"), "aad")
	rotated, _ := NewSealer([]config.DataKey{testKey("k2", 2), testKey("k1", 1)})
	pt, err := rotated.Open(ct, "aad")
	if err != nil || string(pt) != "v" {
		t.Fatalf("old ciphertext must remain readable after rotation: %v", err)
	}
	if !rotated.NeedsRotation(ct) {
		t.Fatal("expected rotation needed")
	}
	newCT, _ := rotated.Seal([]byte("v"), "aad")
	if rotated.NeedsRotation(newCT) {
		t.Fatal("new ciphertext uses primary key")
	}
	if _, err := old.Open(newCT, "aad"); err == nil {
		t.Fatal("old sealer must not know the new key")
	}
}

func TestSealerEmpty(t *testing.T) {
	s, _ := NewSealer([]config.DataKey{testKey("k1", 1)})
	ct, err := s.Seal(nil, "x")
	if err != nil || ct != nil {
		t.Fatal("empty plaintext should seal to nil")
	}
	pt, err := s.Open(nil, "x")
	if err != nil || pt != nil {
		t.Fatal("nil should open to nil")
	}
	_ = base64.StdEncoding
}
