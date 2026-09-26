package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// manifest writes an appmanifest for app id into the Steam folder d and
// creates its install folder.
func manifest(t *testing.T, d string, id, flags int) string {
	t.Helper()
	name := fmt.Sprintf("Game%d", id)
	write(t, filepath.Join(d, "steamapps", fmt.Sprintf("appmanifest_%d.acf", id)),
		fmt.Sprintf(`"AppState" { "appid" "%d" "name" "%s" "StateFlags" "%d" "installdir" "%s" }`, id, name, flags, name))
	dir := filepath.Join(d, "steamapps", "common", name)
	write(t, filepath.Join(dir, "game.exe"), "x")
	return dir
}

func TestAccounts(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "config", "loginusers.vdf"), `"users" {
		"76561197960265729" { "AccountName" "first" "Timestamp" "200" }
		"76561197960265730" { "AccountName" "second" "Timestamp" "100" }
		"76561197960265731" { "AccountName" "gone" "Timestamp" "900" }
	}`)
	for _, acc := range []string{"1", "2"} {
		if err := os.MkdirAll(filepath.Join(d, "userdata", acc), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name string
		h    Host
		want []string
	}{
		{"signed in now", Host{ActiveUser: 2}, []string{"2"}},
		{"auto login", Host{AutoLogin: "SECOND"}, []string{"2"}},
		{"active user without userdata", Host{ActiveUser: 3, AutoLogin: "first"}, []string{"1"}},
		{"last sign-in", Host{}, []string{"1"}},
	} {
		if got := Accounts(d, tt.h); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: accounts = %v, want %v", tt.name, got, tt.want)
		}
	}
	// Older Steam: MostRecent wins over timestamps.
	write(t, filepath.Join(d, "config", "loginusers.vdf"), `"users" {
		"76561197960265729" { "MostRecent" "0" "Timestamp" "200" }
		"76561197960265730" { "MostRecent" "1" "Timestamp" "100" }
	}`)
	if got := Accounts(d, Host{}); !reflect.DeepEqual(got, []string{"2"}) {
		t.Errorf("MostRecent: accounts = %v", got)
	}
}

func TestLibraryApps(t *testing.T) {
	d := t.TempDir()
	manifest(t, d, 10, 4)                                         // installed
	manifest(t, d, 20, 1026)                                      // waiting for an update: still installed
	manifest(t, d, 30, 4|2048)                                    // being uninstalled
	manifest(t, d, 40, 1)                                         // uninstalled
	write(t, filepath.Join(d, "steamapps", "appmanifest_50.acf"), // folder missing
		`"AppState" { "appid" "50" "StateFlags" "4" "installdir" "Missing" }`)
	write(t, filepath.Join(d, "steamapps", "appmanifest_60.acf"), // escapes common
		`"AppState" { "appid" "60" "StateFlags" "4" "installdir" "..\\.." }`)
	apps := Apps(d)
	for id, want := range map[int]bool{10: true, 20: true, 30: false, 40: false, 50: false, 60: false} {
		if a, ok := apps[id]; !ok || a.Installed != want {
			t.Errorf("app %d: %+v (found %v), want installed=%v", id, a, ok, want)
		}
	}
	if apps[60].Dir != "" || apps[50].Dir == "" {
		t.Errorf("install dirs: 50=%q 60=%q", apps[50].Dir, apps[60].Dir)
	}
}

func TestTampered(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	d := t.TempDir()
	write(t, filepath.Join(d, "bin", "steam_api64.dll"), "x")
	if m := Tampered(d, yes); m != "" {
		t.Errorf("signed DLL flagged: %q", m)
	}
	if m := Tampered(d, no); !strings.HasPrefix(m, "steam_api64.dll") {
		t.Errorf("unsigned DLL: %q", m)
	}
	// DLC unlockers still run through Steam.
	write(t, filepath.Join(d, "bin", "cream_api.ini"), "x")
	if m := Tampered(d, no); m != "" {
		t.Errorf("unlocker flagged: %q", m)
	}
	write(t, filepath.Join(d, "bin", "steam_emu.ini"), "x")
	if m := Tampered(d, yes); m != "steam_emu.ini" {
		t.Errorf("emulator ini: %q", m)
	}
	g := t.TempDir()
	write(t, filepath.Join(g, "Game", "steam_settings", "steam_appid.txt"), "1")
	if m := Tampered(g, yes); m != "steam_settings" {
		t.Errorf("Goldberg settings: %q", m)
	}
	deep := t.TempDir()
	write(t, filepath.Join(deep, "Engine", "Binaries", "ThirdParty", "Steamworks", "Steamv157", "Win64", "steam_api64.dll"), "x")
	if m := Tampered(deep, no); !strings.HasPrefix(m, "steam_api64.dll") {
		t.Errorf("Unreal's Steamworks DLL not checked: %q", m)
	}
	write(t, filepath.Join(deep, "a", "b", "c", "d", "e", "f", "g", "h", "steam_emu.ini"), "x")
	if m := Tampered(deep, yes); m != "" {
		t.Errorf("too deep to search, got %q", m)
	}
	if Tampered("", no) != "" {
		t.Error("no folder should be clean")
	}
}
