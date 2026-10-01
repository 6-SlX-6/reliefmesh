package validation

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	e := Errors{}
	e.Text("a", "", 1, 10, false)
	e.Text("b", strings.Repeat("x", 11), 0, 10, false)
	e.Text("c", "line\nbreak", 0, 20, false)
	e.Text("d", "line\nbreak", 0, 20, true)
	e.Text("e", "bell\a", 0, 20, true)
	e.Text("f", "Ünïcödé ok", 1, 10, false)
	for _, f := range []string{"a", "b", "c", "e"} {
		if !e.Has(f) {
			t.Errorf("expected error for %s", f)
		}
	}
	for _, f := range []string{"d", "f"} {
		if e.Has(f) {
			t.Errorf("unexpected error for %s: %s", f, e[f])
		}
	}
}

func TestTagsAndUsername(t *testing.T) {
	e := Errors{}
	tags := e.Tags("tags", []string{" Water ", "water", "north-side"})
	if len(tags) != 2 || tags[0] != "water" {
		t.Fatalf("unexpected tags %v", tags)
	}
	e.Tags("bad", []string{"<script>"})
	if !e.Has("bad") {
		t.Fatal("invalid tag accepted")
	}
	if u := e.Username("u", " Alice@Example.org "); u != "alice@example.org" || e.Has("u") {
		t.Fatalf("username normalization failed: %q %v", u, e)
	}
	e.Username("u2", "a b")
	if !e.Has("u2") {
		t.Fatal("invalid username accepted")
	}
}
