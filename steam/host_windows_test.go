package steam

import (
	"os"
	"path/filepath"
	"sync"
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

// Signed is called from several goroutines by the consumers; the cache must
// hold up (run with -race) and give the same answer every time.
func TestSignedConcurrent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Spiel Ü 游戏")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(dir, "steam_api.dll"), filepath.Join(dir, "steam_api64.dll"), filepath.Join(dir, "missing.dll")}
	write(t, paths[0], "MZ one")
	write(t, paths[1], "MZ two")
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			if p := paths[i%len(paths)]; Signed(p) {
				t.Errorf("Signed(%q) = true", p)
			}
		})
	}
	wg.Wait()
}
