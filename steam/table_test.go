package steam

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// steamDir lays out a Steam folder: loginusers.vdf (if users isn't "") and
// a userdata folder per account.
func steamDir(t *testing.T, users string, userdata ...string) string {
	t.Helper()
	d := t.TempDir()
	if users != "" {
		write(t, filepath.Join(d, "config", "loginusers.vdf"), users)
	}
	for _, acc := range userdata {
		if err := os.MkdirAll(filepath.Join(d, "userdata", acc), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestAccountsTable(t *testing.T) {
	const two = `"users" {
		"76561197960265729" { "AccountName" "first" "Timestamp" "200" }
		"76561197960265730" { "AccountName" "second" "Timestamp" "100" }
	}`
	for _, tt := range []struct {
		name     string
		users    string
		userdata []string
		files    []string // files (not folders) in userdata
		h        Host
		want     []string
	}{
		{"active user wins over everything", two, []string{"1", "2"}, nil, Host{ActiveUser: 2, AutoLogin: "first"}, []string{"2"}},
		{"active user not in loginusers", two, []string{"1", "2", "9"}, nil, Host{ActiveUser: 9}, []string{"9"}},
		{"active user without userdata falls through", two, []string{"1", "2"}, nil, Host{ActiveUser: 3}, []string{"1"}},
		{"auto login, case-insensitive", two, []string{"1", "2"}, nil, Host{AutoLogin: "Second"}, []string{"2"}},
		{"unknown auto login falls through", two, []string{"1", "2"}, nil, Host{AutoLogin: "nobody"}, []string{"1"}},
		{"auto login without userdata falls through", two, []string{"1"}, nil, Host{AutoLogin: "second"}, []string{"1"}},
		{"latest timestamp", two, []string{"1", "2"}, nil, Host{}, []string{"1"}},
		{"MostRecent wins over timestamps", `"users" {
			"76561197960265729" { "MostRecent" "0" "Timestamp" "200" }
			"76561197960265730" { "MostRecent" "1" "Timestamp" "100" }
		}`, []string{"1", "2"}, nil, Host{}, []string{"2"}},
		{"no timestamps: every known account", `"users" {
			"76561197960265730" { "AccountName" "b" }
			"76561197960265729" { "AccountName" "a" }
		}`, []string{"1", "2"}, nil, Host{}, []string{"1", "2"}},
		{"ignores ids that aren't accounts", `"users" {
			"76561197960265728" { "Timestamp" "900" }
			"12" { "Timestamp" "900" }
			"not-a-number" { "Timestamp" "900" }
			"76561197960265729" { "Timestamp" "1" }
		}`, []string{"0", "1", "12"}, nil, Host{}, []string{"1"}},
		{"bad timestamp counts as none", `"users" {
			"76561197960265729" { "Timestamp" "soon" }
			"76561197960265730" { "Timestamp" "5" }
		}`, []string{"1", "2"}, nil, Host{}, []string{"2"}},
		{"no loginusers: userdata folders", "", []string{"0", "7", "42", "-3", "+5", "anonymous"}, []string{"99"}, Host{}, []string{"42", "7"}},
		{"loginusers without userdata: userdata folders", two, []string{"5"}, nil, Host{}, []string{"5"}},
		{"nothing at all", "", nil, nil, Host{}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := steamDir(t, tt.users, tt.userdata...)
			for _, f := range tt.files {
				write(t, filepath.Join(d, "userdata", f), "x")
			}
			if got := Accounts(d, tt.h); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accounts = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLibraries(t *testing.T) {
	d := t.TempDir()
	other := filepath.Join(t.TempDir(), "Lib")
	old := filepath.Join(t.TempDir(), "OldLib")
	vdfPath := func(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }
	write(t, filepath.Join(d, "steamapps", "libraryfolders.vdf"), `"LibraryFolders" {
		"contentstatsid" "123"
		"0" { "path" "`+vdfPath(d)+`" }
		"1" { "path" "`+vdfPath(other)+`" }
		"2" { "path" "`+vdfPath(strings.ToUpper(other))+`" }
		"3" { "path" "relative\\lib" }
		"4" { "path" "" }
		"5" "`+vdfPath(old)+`"
	}`)
	got := Libraries(d)
	if len(got) != 3 || got[0] != filepath.Clean(d) {
		t.Fatalf("libraries = %q", got)
	}
	rest := map[string]bool{strings.ToLower(got[1]): true, strings.ToLower(got[2]): true}
	if !rest[strings.ToLower(other)] || !rest[strings.ToLower(old)] {
		t.Errorf("libraries = %q, want %q and %q after Steam's folder", got, other, old)
	}
	for i := 0; i < 20; i++ {
		if again := Libraries(d); !reflect.DeepEqual(again, got) {
			t.Fatalf("not stable: %q then %q", got, again)
		}
	}
	if Libraries("relative") != nil {
		t.Error("a relative Steam folder must give no libraries")
	}
	if got := Libraries(t.TempDir()); len(got) != 1 {
		t.Errorf("no libraryfolders.vdf: %q", got)
	}
}

func TestInstallDir(t *testing.T) {
	for _, tt := range []struct {
		installdir string
		ok         bool
	}{
		{"Game", true},
		{"Game With Spaces", true},
		{`Studio\Game`, true},
		{"", false},
		{".", false},
		{"..", false},
		{`..\..`, false},
		{`Game\..`, false},
		{`Game\..\..\x`, false},
		{`C:\Windows`, false},
		{`\\server\share`, false},
		{`\Windows`, false},
		{"/etc", false},
	} {
		t.Run(tt.installdir, func(t *testing.T) {
			if runtime.GOOS != "windows" && strings.ContainsAny(tt.installdir, `\:`) {
				t.Skip("Windows path")
			}
			d := t.TempDir()
			write(t, filepath.Join(d, "steamapps", "appmanifest_1.acf"),
				`"AppState" { "appid" "1" "StateFlags" "4" "installdir" "`+strings.ReplaceAll(tt.installdir, `\`, `\\`)+`" }`)
			if tt.ok {
				write(t, filepath.Join(d, "steamapps", "common", filepath.FromSlash(strings.ReplaceAll(tt.installdir, `\`, "/")), "game.exe"), "x")
			}
			apps := LibraryApps(d)
			if len(apps) != 1 {
				t.Fatalf("apps = %+v", apps)
			}
			if got := apps[0].Dir != ""; got != tt.ok || apps[0].Installed != tt.ok {
				t.Errorf("dir = %q installed = %v, want ok=%v", apps[0].Dir, apps[0].Installed, tt.ok)
			}
		})
	}
}

func TestStateFlags(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags string
		want  bool
	}{
		{"fully installed", "4", true},
		{"update required", "6", true},
		{"update required only", "2", true},
		{"update running", "1026", true},
		{"update paused", "516", true},
		{"update started", "1028", true},
		{"uninstalled", "1", false},
		{"invalid", "0", false},
		{"uninstalling", "2052", false},
		{"not a number", "lots", false},
		{"missing", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := t.TempDir()
			write(t, filepath.Join(d, "steamapps", "appmanifest_1.acf"),
				`"AppState" { "appid" "1" "StateFlags" "`+tt.flags+`" "installdir" "G" }`)
			write(t, filepath.Join(d, "steamapps", "common", "G", "g.exe"), "x")
			if apps := LibraryApps(d); len(apps) != 1 || apps[0].Installed != tt.want {
				t.Errorf("apps = %+v, want installed=%v", apps, tt.want)
			}
		})
	}
}

func TestAppsAcrossLibraries(t *testing.T) {
	d := t.TempDir()
	lib := t.TempDir()
	write(t, filepath.Join(d, "steamapps", "libraryfolders.vdf"),
		`"libraryfolders" { "1" { "path" "`+strings.ReplaceAll(lib, `\`, `\\`)+`" } }`)
	manifest(t, d, 10, 1)   // left behind uninstalled in Steam's folder…
	manifest(t, lib, 10, 4) // …and installed in the other library
	manifest(t, d, 20, 4)
	manifest(t, lib, 20, 1) // an installed copy isn't replaced by an uninstalled one
	apps := Apps(d)
	if a := apps[10]; !a.Installed || !within(lib, a.Dir) {
		t.Errorf("app 10: %+v", a)
	}
	if a := apps[20]; !a.Installed || !within(d, a.Dir) {
		t.Errorf("app 20: %+v", a)
	}
	if len(Apps("")) != 0 {
		t.Error("no Steam folder: no apps")
	}
}

// TestLibraryAppsGolden reads a library laid out as Steam does it.
func TestLibraryAppsGolden(t *testing.T) {
	lib := filepath.Join("testdata", "library")
	apps := LibraryApps(lib)
	for i := range apps {
		if apps[i].Dir != "" {
			rel, err := filepath.Rel(lib, apps[i].Dir)
			if err != nil {
				t.Fatal(err)
			}
			apps[i].Dir = filepath.ToSlash(rel)
		}
	}
	got, err := json.MarshalIndent(apps, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	p := filepath.Join("testdata", "library.golden")
	if *update {
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("library apps differ from %s (go test -update rewrites it):\n%s", p, got)
	}
}

func TestTamperedTable(t *testing.T) {
	signed := func(string) bool { return true }
	unsigned := func(string) bool { return false }
	onlyVendor := func(p string) bool { return strings.Contains(p, "vendor") }
	for _, tt := range []struct {
		name   string
		files  []string
		signed func(string) bool
		want   string // prefix of the result; "" means clean
	}{
		{"clean game", []string{"game.exe", "data/pak0.pak"}, unsigned, ""},
		{"signed steam_api", []string{"steam_api.dll"}, signed, ""},
		{"unsigned steam_api", []string{"steam_api.dll"}, unsigned, "steam_api.dll (unsigned or altered)"},
		{"no signature check", []string{"steam_api64.dll"}, nil, ""},
		{"any unsigned copy counts", []string{"vendor/steam_api64.dll", "bin/steam_api64.dll"}, onlyVendor, "steam_api64.dll (unsigned"},
		{"DLL name in other case", []string{"Steam_API64.DLL"}, unsigned, "Steam_API64.DLL"},
		{"CODEX", []string{"steam_emu.ini"}, signed, "steam_emu.ini"},
		{"CODEX renamed DLL", []string{"bin/steam_api64.cdx"}, signed, "steam_api64.cdx"},
		{"RUNE", []string{"steam_api64.rne"}, signed, "steam_api64.rne"},
		{"Goldberg interfaces", []string{"steam_interfaces.txt"}, signed, "steam_interfaces.txt"},
		{"Goldberg local saves", []string{"local_save.txt"}, signed, "local_save.txt"},
		{"Goldberg settings folder", []string{"steam_settings/steam_appid.txt"}, signed, "steam_settings"},
		{"Goldberg settings folder, other case", []string{"bin/Steam_Settings/x.txt"}, signed, "Steam_Settings"},
		{"GSE loader", []string{"ColdClientLoader.ini"}, signed, "ColdClientLoader.ini"},
		{"OnlineFix", []string{"OnlineFix64.dll"}, signed, "OnlineFix64.dll"},
		{"SmartSteamEmu", []string{"SmartSteamEmu.ini"}, signed, "SmartSteamEmu.ini"},
		{"CreamAPI keeps Steam", []string{"cream_api.ini", "steam_api.dll"}, unsigned, ""},
		{"SmokeAPI keeps Steam", []string{"SmokeAPI.config.json", "steam_api64.dll", "steam_api64_o.dll"}, unsigned, ""},
		{"unlocker plus emulator", []string{"cream_api.ini", "steam_emu.ini"}, signed, "steam_emu.ini"},
		{"marker as a folder name isn't a marker", []string{"steam_emu.ini/readme.txt"}, signed, ""},
		{"DLL at the depth limit", []string{"1/2/3/4/5/6/steam_api.dll"}, unsigned, "steam_api.dll"},
		{"marker below the depth limit", []string{"1/2/3/4/5/6/7/steam_emu.ini"}, signed, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := t.TempDir()
			for _, f := range tt.files {
				write(t, filepath.Join(d, filepath.FromSlash(f)), "x")
			}
			got := Tampered(d, tt.signed)
			if (tt.want == "") != (got == "") || !strings.HasPrefix(got, tt.want) {
				t.Errorf("Tampered = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTamperedMissingFolder(t *testing.T) {
	if got := Tampered(filepath.Join(t.TempDir(), "gone"), func(string) bool { return false }); got != "" {
		t.Errorf("missing folder: %q", got)
	}
}

func TestWithin(t *testing.T) {
	sep := string(filepath.Separator)
	for _, tt := range []struct {
		parent, child string
		want          bool
	}{
		{"a", "a", true},
		{"a", "a" + sep + "b", true},
		{"a", "A" + sep + "B", true},
		{"a", "ab", false},
		{"a" + sep + "b", "a", false},
		{sep, sep + "x", true},
	} {
		if got := within(tt.parent, tt.child); got != tt.want {
			t.Errorf("within(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.want)
		}
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"":           "",
		"   ":        "",
		" c:/steam ": filepath.Clean(filepath.FromSlash("c:/steam")),
		"a/b/../c":   filepath.Clean(filepath.FromSlash("a/c")),
	} {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}
