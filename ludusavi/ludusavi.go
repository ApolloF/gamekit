// Package ludusavi reads the community Ludusavi manifest: PCGamingWiki data
// on ~50,000 PC games with their Steam and GOG ids, the folder names they
// install into, where they keep their saves, and whether they use Steam
// Cloud.
package ludusavi

import (
	"bufio"
	"io"
	"slices"
	"strconv"
	"strings"
)

// URL is where the manifest is published.
const URL = "https://raw.githubusercontent.com/mtkennerly/ludusavi-manifest/master/data/manifest.yaml"

// Entry is one game in the manifest.
type Entry struct {
	Name        string
	SteamID     int      // Steam app id, 0 if not on Steam
	GogID       string   // GOG product id, "" if not on GOG
	InstallDirs []string // folder names the game installs into
	Aliases     []string // other names in the manifest that point to this game
	Saves       []string // Windows save locations, with manifest placeholders (<winAppData>, <base>, …)
	SteamCloud  bool     // the game supports Steam Cloud (not that it's in use)
	UplayCloud  bool     // the game supports Ubisoft Connect cloud saves

	// RootSaves are Windows save locations inside a store's own folder
	// (<root>/…), such as Ubisoft Connect's savegames/<storeUserId>/<id>.
	// They are kept apart from Saves because <root> means a different
	// folder for every store.
	RootSaves []RootPath
}

// RootPath is a save location below a store's folder (<root>) and the stores
// it applies to (empty: any store).
type RootPath struct {
	Path   string   // starts with "<root>/"
	Stores []string // store ids as the manifest names them: "steam", "uplay", "gog", "epic", "microsoft"
}

// Parse reads the manifest YAML. Alias entries (another name for a game)
// aren't returned on their own but listed in the game's Aliases.
//
// The file is machine-generated with a fixed two-space layout, so a line
// scanner is used instead of a generic YAML decoder: it is ~20x faster and
// needs a few MB instead of hundreds.
func Parse(r io.Reader) ([]Entry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var (
		out     []Entry
		cur     *Entry
		alias   string
		section string
		p       *pathInfo
	)
	aliases := map[string][]string{} // game name → alias names
	flush := func() {
		if p != nil && cur != nil && p.relevant() {
			cur.Saves = append(cur.Saves, p.path)
		} else if p != nil && cur != nil && p.rootRelevant() {
			cur.RootSaves = append(cur.RootSaves, RootPath{Path: p.path, Stores: p.windowsStores()})
		}
		p = nil
	}
	finish := func() {
		flush()
		if cur == nil {
			return
		}
		if alias != "" {
			aliases[alias] = append(aliases[alias], cur.Name)
		} else {
			out = append(out, *cur)
		}
		cur, alias = nil, ""
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line == "---" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		ind := len(line) - len(strings.TrimLeft(line, " "))
		t := strings.TrimSpace(line)
		switch {
		case ind == 0:
			finish()
			cur = &Entry{Name: unquote(strings.TrimSuffix(t, ":"))}
			section = ""
		case cur == nil:
		case ind == 2:
			flush()
			k, v, _ := strings.Cut(t, ":")
			section = k
			if k == "alias" {
				alias = unquote(strings.TrimSpace(v))
			}
		case ind == 4 && section == "cloud":
			if k, v, ok := strings.Cut(t, ":"); ok && strings.TrimSpace(v) == "true" {
				switch strings.TrimSpace(k) {
				case "steam":
					cur.SteamCloud = true
				case "uplay":
					cur.UplayCloud = true
				}
			}
		case ind == 4 && section == "installDir":
			if dir := unquote(strings.TrimSuffix(strings.TrimSuffix(t, " {}"), ":")); dir != "" {
				cur.InstallDirs = append(cur.InstallDirs, dir)
			}
		case ind == 4 && (section == "steam" || section == "gog"):
			k, v, ok := strings.Cut(t, ":")
			if !ok || k != "id" {
				continue
			}
			v = strings.TrimSpace(v)
			if section == "steam" {
				if id, err := strconv.Atoi(v); err == nil && id > 0 {
					cur.SteamID = id
				}
			} else if _, err := strconv.ParseInt(v, 10, 64); err == nil {
				cur.GogID = v
			}
		case section == "files" && ind == 4:
			flush()
			p = &pathInfo{path: unquote(strings.TrimSuffix(strings.TrimSuffix(t, " {}"), ":"))}
		case section == "files" && p != nil && ind == 6:
			p.sub = strings.TrimSuffix(t, ":")
		case section == "files" && p != nil && ind >= 8:
			v := strings.TrimSpace(strings.TrimPrefix(t, "- "))
			switch p.sub {
			case "tags":
				p.tags = append(p.tags, v)
			case "when":
				if strings.HasPrefix(t, "- ") || len(p.whens) == 0 {
					p.whens = append(p.whens, condition{})
				}
				w := &p.whens[len(p.whens)-1]
				if k, val, ok := strings.Cut(v, ":"); ok {
					switch strings.TrimSpace(k) {
					case "os":
						w.os = strings.TrimSpace(val)
					case "store":
						w.store = strings.TrimSpace(val)
					}
				}
			}
		}
	}
	finish()
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Aliases = aliases[out[i].Name]
	}
	return out, nil
}

// pathInfo is one entry under "files" while it is being read.
type pathInfo struct {
	path  string
	sub   string
	tags  []string
	whens []condition
}

// condition is one item of a path's "when" list; "" means any.
type condition struct {
	os, store string
}

// onWindows reports whether the condition can hold on Windows.
func (c condition) onWindows() bool { return c.os == "" || c.os == "windows" }

var winPrefixes = []string{"<winAppData>", "<winLocalAppData>", "<winDocuments>", "<home>", "<winPublic>", "<winProgramData>"}

// relevant reports whether a path holds saves on Windows.
func (p *pathInfo) relevant() bool {
	ok := false
	for _, pre := range winPrefixes {
		if strings.HasPrefix(p.path, pre) {
			ok = safeBelow(p.path, pre)
			break
		}
	}
	return ok && p.saveTagged() && p.windowsOK()
}

// rootRelevant reports whether a path below a store's folder holds saves on
// Windows.
func (p *pathInfo) rootRelevant() bool {
	return strings.HasPrefix(p.path, "<root>/") && safeBelow(p.path, "<root>") && p.saveTagged() && p.windowsOK()
}

// safeBelow reports whether path, which starts with the placeholder pre,
// stays below it. The manifest is community-edited, so a path may try to
// climb out ("<winAppData>/../../Windows"), or switch to another root
// ("<winAppData>C:/x", "<winAppData>//server/share").
func safeBelow(path, pre string) bool {
	rest := strings.TrimPrefix(path, pre)
	if rest == "" {
		return true
	}
	if rest[0] != '/' || strings.HasPrefix(rest, "//") || strings.Contains(rest, ":") {
		return false
	}
	for _, seg := range strings.FieldsFunc(rest, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return false
		}
	}
	return true
}

// saveTagged reports whether the path is tagged as saves (untagged counts).
func (p *pathInfo) saveTagged() bool {
	if len(p.tags) == 0 {
		return true
	}
	for _, t := range p.tags {
		if t == "save" {
			return true
		}
	}
	return false
}

// windowsOK reports whether the path applies on Windows: it has no
// conditions, or one of them allows Windows (store-only conditions apply
// everywhere).
func (p *pathInfo) windowsOK() bool {
	if len(p.whens) == 0 {
		return true
	}
	for _, c := range p.whens {
		if c.onWindows() {
			return true
		}
	}
	return false
}

// windowsStores lists the stores the path applies to on Windows; nil if any.
// A condition for another OS doesn't restrict the stores on Windows.
func (p *pathInfo) windowsStores() []string {
	var stores []string
	for _, c := range p.whens {
		if !c.onWindows() {
			continue
		}
		if c.store == "" {
			return nil
		}
		if !slices.Contains(stores, c.store) {
			stores = append(stores, c.store)
		}
	}
	return stores
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return s
}
