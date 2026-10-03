package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSigned(t *testing.T) {
	unsigned := filepath.Join(t.TempDir(), "steam_api64.dll")
	write(t, unsigned, "MZ not really a DLL")
	if Signed(unsigned) || Signed(unsigned) { // second call is served from the cache
		t.Error("an unsigned file must not count as signed")
	}
	if Signed(filepath.Join(t.TempDir(), "missing.dll")) || Signed("bad\x00path") {
		t.Error("a missing file must not count as signed")
	}
	// A file with an embedded signature, if this PC has one of them.
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	if d := Dir(); d != "" {
		candidates = append([]string{filepath.Join(d, "steam.exe")}, candidates...)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if !Signed(p) {
				t.Errorf("%s should be signed", p)
			}
			return
		}
	}
	t.Log("no signed file to check against")
}

// The registry readers depend on this PC; they must not fail without Steam.
func TestHostSmoke(t *testing.T) {
	if d := Dir(); d != "" && !isDir(filepath.Join(d, "userdata")) {
		t.Errorf("Dir() = %q has no userdata", d)
	}
	h := HostInfo()
	if h.Running == nil || h.Signed == nil {
		t.Error("HostInfo must fill in Running and Signed")
	}
	if Running(-1) {
		t.Error("app -1 can't be running")
	}
}
