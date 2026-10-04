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
	// Signed means signed by Valve, so only a Valve binary can pass: Steam's
	// own steam.exe, when Steam is installed (it isn't on CI).
	d := Dir()
	if d == "" {
		t.Skip("Steam isn't installed: no Valve-signed file to check against")
	}
	steamExe := filepath.Join(d, "steam.exe")
	if _, err := os.Stat(steamExe); err != nil {
		t.Skip("no steam.exe to check against")
	}
	if !Signed(steamExe) {
		t.Errorf("%s should be signed", steamExe)
	}
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
