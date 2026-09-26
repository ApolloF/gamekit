package ludusavi

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sample = `---
Stardew Valley:
  cloud:
    gog: true
    steam: true
  files:
    "<home>/.config/StardewValley/Saves":
      tags:
        - save
      when:
        - os: mac
    "<winAppData>/StardewValley/Saves":
      tags:
        - save
      when:
        - os: windows
    "<winAppData>/StardewValley/default_options":
      tags:
        - config
      when:
        - os: windows
  installDir:
    Stardew Valley: {}
    "Other Dir": {}
    'Game: It''s a Dir':
  steam:
    id: 413150
"Game: With Colon":
  files:
    <winDocuments>/My Games/Colon/<storeUserId>:
      tags:
        - save
      when:
        - store: steam
    <base>/saves:
      tags:
        - save
Hades:
  gog:
    id: 1330167364
  installDir:
    Hades: {}
  steam:
    id: 1145360
"-KLAUS-":
  alias: Klaus
Klaus:
  installDir:
    KLAUS: {}
  steam:
    id: 431330
`

func TestParse(t *testing.T) {
	es, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 4 {
		t.Fatalf("want 4 entries (aliases aren't entries), got %d: %+v", len(es), es)
	}
	sv := es[0]
	if sv.Name != "Stardew Valley" || !sv.SteamCloud || sv.SteamID != 413150 || !reflect.DeepEqual(sv.Saves, []string{"<winAppData>/StardewValley/Saves"}) {
		t.Errorf("stardew: %+v", sv)
	}
	if !reflect.DeepEqual(sv.InstallDirs, []string{"Stardew Valley", "Other Dir", "Game: It's a Dir"}) {
		t.Errorf("stardew install dirs: %q", sv.InstallDirs)
	}
	c := es[1]
	if c.Name != "Game: With Colon" || c.SteamCloud || c.SteamID != 0 || !reflect.DeepEqual(c.Saves, []string{"<winDocuments>/My Games/Colon/<storeUserId>"}) {
		t.Errorf("colon: %+v", c)
	}
	if len(c.InstallDirs) != 0 {
		t.Errorf("install dirs leaked between entries: %+v", c)
	}
	if h := es[2]; h.GogID != "1330167364" || h.SteamID != 1145360 || len(h.Saves) != 0 {
		t.Errorf("hades: %+v", h)
	}
	if k := es[3]; k.Name != "Klaus" || !reflect.DeepEqual(k.Aliases, []string{"-KLAUS-"}) {
		t.Errorf("klaus: %+v", k)
	}
}

func TestParseInvalidIDs(t *testing.T) {
	for _, id := range []string{"nope", "-1", "0", "999999999999999999999999"} {
		es, err := Parse(strings.NewReader("Game:\n  steam:\n    id: " + id + "\n  gog:\n    id: " + id + "\n"))
		if err != nil || len(es) != 1 || es[0].SteamID != 0 {
			t.Errorf("id %q: entries=%+v err=%v", id, es, err)
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add(sample)
	f.Fuzz(func(t *testing.T, s string) { _, _ = Parse(strings.NewReader(s)) })
}

// TestParseReal runs against the full manifest when LUDUSAVI_MANIFEST points at it.
func TestParseReal(t *testing.T) {
	p := os.Getenv("LUDUSAVI_MANIFEST")
	if p == "" {
		t.Skip("LUDUSAVI_MANIFEST not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	start := time.Now()
	es, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	withSaves := 0
	for _, e := range es {
		if len(e.Saves) > 0 {
			withSaves++
		}
	}
	t.Logf("parsed %d entries (%d with Windows saves) in %v", len(es), withSaves, time.Since(start))
}
