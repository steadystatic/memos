package identifier

import "testing"

func TestSpaceKeyMatcher(t *testing.T) {
	for input, want := range map[string]bool{
		"haymakr": true, "a": true, "my-space-2": true, "0123456789abcdef": true,
		"": false, "-a": false, "a-": false, "Haymakr": false, "has space": false, "0123456789abcdefg": false,
	} {
		if got := SpaceKeyMatcher.MatchString(input); got != want {
			t.Errorf("SpaceKeyMatcher(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseHandle(t *testing.T) {
	for _, tc := range []struct {
		in   string
		key  string
		seq  int32
		good bool
	}{
		{"haymakr-12", "haymakr", 12, true},
		{"Haymakr-12", "haymakr", 12, true},
		{" haymakr-7 ", "haymakr", 7, true},
		{"my-space-3", "my-space", 3, true},
		{"a-1-5", "a-1", 5, true},
		{"haymakr", "", 0, false},
		{"haymakr-", "", 0, false},
		{"-12", "", 0, false},
		{"haymakr-0", "", 0, false},
		{"haymakr--1", "", 0, false},
		{"haymakr-x", "", 0, false},
		{"haymakr-99999999999", "", 0, false},
		{"bad key-1", "", 0, false},
	} {
		key, seq, ok := ParseHandle(tc.in)
		if ok != tc.good || key != tc.key || seq != tc.seq {
			t.Errorf("ParseHandle(%q) = (%q, %d, %v), want (%q, %d, %v)", tc.in, key, seq, ok, tc.key, tc.seq, tc.good)
		}
	}
}

func TestFormatHandleRoundTrips(t *testing.T) {
	key, seq, ok := ParseHandle(FormatHandle("my-space", 42))
	if !ok || key != "my-space" || seq != 42 {
		t.Fatalf("round trip = (%q, %d, %v)", key, seq, ok)
	}
}

func TestDeriveSpaceKey(t *testing.T) {
	for _, tc := range []struct{ uid, title, want string }{
		{"haymakr", "Haymakr", "haymakr"},
		{"Homelab", "x", "homelab"},
		{"7dda9b2b-ae3a-4692-bf5c-887a5a4f2b10", "Weekly Song Journal", "weekly-song-jour"},
		{"7dda9b2b-ae3a-4692-bf5c-887a5a4f2b10", "Infrastructure & Homelab", "infrastructure-h"},
		{"7dda9b2b-ae3a-4692-bf5c-887a5a4f2b10", "!!!", "space"},
	} {
		got := DeriveSpaceKey(tc.uid, tc.title)
		if got != tc.want {
			t.Errorf("DeriveSpaceKey(%q, %q) = %q, want %q", tc.uid, tc.title, got, tc.want)
		}
		if !SpaceKeyMatcher.MatchString(got) {
			t.Errorf("DeriveSpaceKey(%q, %q) = %q is not a valid key", tc.uid, tc.title, got)
		}
	}
}
