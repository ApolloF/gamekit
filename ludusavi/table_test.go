package ludusavi

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// game builds a manifest with one game whose only file is path, with the
// given tags and "when" lines (already indented as list items).
func game(path string, tags []string, when ...string) string {
	var sb strings.Builder
	sb.WriteString("Game:\n  files:\n    \"" + path + "\":\n")
	if len(tags) > 0 {
		sb.WriteString("      tags:\n")
		for _, t := range tags {
			sb.WriteString("        - " + t + "\n")
		}
	}
	if len(when) > 0 {
		sb.WriteString("      when:\n")
		for _, w := range when {
			sb.WriteString("        " + w + "\n")
		}
	}
	return sb.String()
}

func TestFileConditions(t *testing.T) {
	save := []string{"save"}
	for _, tt := range []struct {
		name  string
		in    string
		saves []string
		roots []RootPath
	}{
		{"untagged, unconditional", game("<winAppData>/G", nil), []string{"<winAppData>/G"}, nil},
		{"config only", game("<winAppData>/G/cfg", []string{"config"}), nil, nil},
		{"save and config", game("<winAppData>/G", []string{"config", "save"}), []string{"<winAppData>/G"}, nil},
		{"windows", game("<winLocalAppData>/G", save, "- os: windows"), []string{"<winLocalAppData>/G"}, nil},
		{"linux only", game("<home>/.local/G", save, "- os: linux"), nil, nil},
		{"mac or linux", game("<home>/G", save, "- os: mac", "- os: linux"), nil, nil},
		{"linux or windows", game("<home>/G", save, "- os: linux", "- os: windows"), []string{"<home>/G"}, nil},
		{"store only applies everywhere", game("<winDocuments>/G", save, "- store: steam"), []string{"<winDocuments>/G"}, nil},
		{"linux, or any OS on steam", game("<home>/G", save, "- os: linux", "- store: steam"), []string{"<home>/G"}, nil},
		{"linux on steam only", game("<home>/G", save, "- os: linux", "  store: steam"), nil, nil},
		{"every Windows prefix", "Game:\n  files:\n" +
			"    <winPublic>/a: {}\n    <winProgramData>/b: {}\n    <winDocuments>/c: {}\n",
			[]string{"<winPublic>/a", "<winProgramData>/b", "<winDocuments>/c"}, nil},
		{"base and game paths aren't Windows save locations", "Game:\n  files:\n    <base>/saves:\n    <game>/x:\n    <xdgData>/y:\n", nil, nil},
		{"root, any store", game("<root>/saves", save), nil, []RootPath{{Path: "<root>/saves"}}},
		{"root, one store", game("<root>/savegames/<storeUserId>/1", save, "- store: uplay"), nil,
			[]RootPath{{Path: "<root>/savegames/<storeUserId>/1", Stores: []string{"uplay"}}}},
		{"root, store and os on one item", game("<root>/userdata/x", save, "- os: windows", "  store: steam"), nil,
			[]RootPath{{Path: "<root>/userdata/x", Stores: []string{"steam"}}}},
		{"root, linux item doesn't restrict Windows stores", game("<root>/s", save, "- os: linux", "  store: steam", "- os: windows", "  store: uplay"), nil,
			[]RootPath{{Path: "<root>/s", Stores: []string{"uplay"}}}},
		{"root, store repeated per OS", game("<root>/s", save, "- os: windows", "  store: steam", "- os: linux", "  store: steam", "- store: steam"), nil,
			[]RootPath{{Path: "<root>/s", Stores: []string{"steam"}}}},
		{"root, any store on Windows wins", game("<root>/s", save, "- store: uplay", "- os: windows"), nil,
			[]RootPath{{Path: "<root>/s"}}},
		{"root, linux only", game("<root>/s", save, "- os: linux", "  store: steam"), nil, nil},
		{"root, config", game("<root>/cfg", []string{"config"}), nil, nil},
		{"root itself isn't a save path", game("<root>", save), nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			es, err := Parse(strings.NewReader(tt.in))
			if err != nil || len(es) != 1 {
				t.Fatalf("entries=%+v err=%v", es, err)
			}
			if !reflect.DeepEqual(es[0].Saves, tt.saves) {
				t.Errorf("saves = %q, want %q", es[0].Saves, tt.saves)
			}
			if !reflect.DeepEqual(es[0].RootSaves, tt.roots) {
				t.Errorf("root saves = %+v, want %+v", es[0].RootSaves, tt.roots)
			}
		})
	}
}

func TestParseEdgeCases(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want []Entry
	}{
		{"empty", "", nil},
		{"only the document marker", "---\n", nil},
		{"comments", "# a\nGame:\n  # b\n  steam:\n    id: 5\n", []Entry{{Name: "Game", SteamID: 5}}},
		{"indented lines before any game", "  steam:\n    id: 5\nGame:\n", []Entry{{Name: "Game"}}},
		{"quoted names", "\"A: B\":\n'It''s':\n\"Tab\\tName\":\n", []Entry{{Name: "A: B"}, {Name: "It's"}, {Name: "Tab\tName"}}},
		{"broken quotes kept", "\"A\\q\":\n", []Entry{{Name: "A\\q"}}},
		{"cloud flags", "G:\n  cloud:\n    epic: true\n    gog: true\n    steam: false\n    uplay: true\n", []Entry{{Name: "G", UplayCloud: true}}},
		{"gog id must be a number", "G:\n  gog:\n    id: abc\n", []Entry{{Name: "G"}}},
		{"other keys under steam", "G:\n  steam:\n    other: 7\n    id: 9\n", []Entry{{Name: "G", SteamID: 9}}},
		{"alias of a later game", "Old:\n  alias: New\nNew:\n", []Entry{{Name: "New", Aliases: []string{"Old"}}}},
		{"two aliases", "A:\n  alias: G\nG:\nB:\n  alias: G\n", []Entry{{Name: "G", Aliases: []string{"A", "B"}}}},
		{"alias of a missing game", "A:\n  alias: Nope\n", nil},
		{"CRLF line endings", "G:\r\n  steam:\r\n    id: 5\r\n", []Entry{{Name: "G", SteamID: 5}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.in))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestParseLineTooLong(t *testing.T) {
	if _, err := Parse(strings.NewReader("G:\n  x: " + strings.Repeat("a", 2<<20) + "\n")); err == nil {
		t.Error("a line over 1 MB must fail")
	}
}

// TestParseGolden reads a slice of the real manifest, covering the shapes it
// uses: aliases, quoting, cloud flags, store folders and per-OS conditions.
func TestParseGolden(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	es, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep <root> and friends readable
	enc.SetIndent("", "\t")
	if err := enc.Encode(es); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	p := filepath.Join("testdata", "manifest.golden.json")
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
		t.Errorf("parsed manifest differs from %s (go test -update rewrites it):\n%s", p, got)
	}
}
