package ludusavi

import (
	"os"
	"reflect"
	"slices"
	"strconv"
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
	f.Add(ubisoftSample)
	f.Fuzz(func(t *testing.T, s string) {
		es, err := Parse(strings.NewReader(s))
		if err != nil {
			return
		}
		for _, e := range es {
			if e.SteamID < 0 {
				t.Fatalf("%q: steam id %d", e.Name, e.SteamID)
			}
			if _, err := strconv.ParseInt(e.GogID, 10, 64); e.GogID != "" && err != nil {
				t.Fatalf("%q: gog id %q", e.Name, e.GogID)
			}
			for _, p := range e.Saves {
				if !slices.ContainsFunc(winPrefixes, func(pre string) bool { return strings.HasPrefix(p, pre) }) {
					t.Fatalf("%q: save path %q isn't a Windows location", e.Name, p)
				}
			}
			for _, r := range e.RootSaves {
				if !strings.HasPrefix(r.Path, "<root>/") {
					t.Fatalf("%q: root save %q", e.Name, r.Path)
				}
				for i, st := range r.Stores {
					if st == "" || slices.Contains(r.Stores[:i], st) {
						t.Fatalf("%q: stores %q", e.Name, r.Stores)
					}
				}
			}
		}
	})
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

const ubisoftSample = `---
"Assassin's Creed Odyssey":
  cloud:
    uplay: true
  files:
    "<root>/savegames/<storeUserId>/5059":
      tags:
        - save
      when:
        - store: uplay
    "<root>/savegames/<storeUserId>/5092":
      tags:
        - save
      when:
        - store: steam
        - store: uplay
    "<root>/userdata/<storeUserId>/812140/remote":
      tags:
        - save
      when:
        - os: windows
          store: steam
    "<root>/linux-only":
      tags:
        - save
      when:
        - os: linux
    "<root>/config.ini":
      tags:
        - config
    "<winDocuments>/Assassin's Creed Odyssey/ACOdyssey.ini":
      tags:
        - config
      when:
        - os: windows
  steam:
    id: 812140
`

func TestParseRootSaves(t *testing.T) {
	es, err := Parse(strings.NewReader(ubisoftSample))
	if err != nil || len(es) != 1 {
		t.Fatalf("entries=%+v err=%v", es, err)
	}
	e := es[0]
	if !e.UplayCloud || e.SteamCloud || len(e.Saves) != 0 {
		t.Errorf("flags/saves: %+v", e)
	}
	want := []RootPath{
		{Path: "<root>/savegames/<storeUserId>/5059", Stores: []string{"uplay"}},
		{Path: "<root>/savegames/<storeUserId>/5092", Stores: []string{"steam", "uplay"}},
		{Path: "<root>/userdata/<storeUserId>/812140/remote", Stores: []string{"steam"}},
	}
	if !reflect.DeepEqual(e.RootSaves, want) {
		t.Errorf("root saves:\n got %+v\nwant %+v", e.RootSaves, want)
	}
}
