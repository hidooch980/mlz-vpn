package lan

import "testing"

func TestLANHelpers(t *testing.T) {
	pin := GeneratePIN()
	if len(pin) != 4 {
		t.Fatalf("expected 4-digit PIN, got %q", pin)
	}
	for _, ch := range pin {
		if ch < '0' || ch > '9' {
			t.Fatalf("expected numeric PIN, got %q", pin)
		}
	}

	primary, list := DetectLANAddresses()
	if primary == "" || len(list) == 0 {
		t.Fatalf("expected non-empty primary IP fallback, got primary=%q list=%v", primary, list)
	}
}
