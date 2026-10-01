package export

import "testing"

func TestSafeCell(t *testing.T) {
	for in, want := range map[string]string{"=SUM(A1)": "'=SUM(A1)", "+1": "'+1", "-2": "'-2", "@cmd": "'@cmd", "ok": "ok", "": ""} {
		if got := SafeCell(in); got != want {
			t.Errorf("SafeCell(%q)=%q want %q", in, got, want)
		}
	}
}
