package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSignedBy(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	name := func(s string) func(string) string { return func(string) string { return s } }
	for _, tc := range []struct {
		desc    string
		trusted func(string) bool
		signer  string
		want    bool
	}{
		{"valve", yes, "Valve Corp.", true},
		{"valve, case and spaces", yes, " valve corp. ", true},
		{"other valid signer", yes, "Cheap Cert Ltd", false},
		{"lookalike signer", yes, "Valve Corp. Fake", false},
		{"no signer", yes, "", false},
		{"valve name but not trusted", no, "Valve Corp.", false},
	} {
		if got := signedBy("x.dll", tc.trusted, name(tc.signer), "Valve Corp."); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.desc, got, tc.want)
		}
	}
}

func TestSignerNameUnsigned(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.dll")
	if err := os.WriteFile(p, []byte("MZ not signed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := signerName(p); got != "" {
		t.Errorf("signerName(unsigned) = %q", got)
	}
}

// A signed system binary has a signer name, but it isn't Valve, so Signed
// must refuse it.
func TestSignedRejectsNonValveSigner(t *testing.T) {
	p := filepath.Join(os.Getenv("SystemRoot"), "System32", "kernel32.dll")
	if _, err := os.Stat(p); err != nil {
		t.Skip("no kernel32.dll")
	}
	name := signerName(p)
	if name == "" {
		t.Skip("kernel32.dll is catalog-signed here")
	}
	if Signed(p) {
		t.Errorf("Signed(kernel32.dll) = true, signer %q", name)
	}
}
