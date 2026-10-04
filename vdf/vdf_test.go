package vdf

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	n := Parse(strings.NewReader(`
"libraryfolders"
{
	// comment
	"0"
	{
		"path"		"C:\\Program Files (x86)\\Steam"
		"apps" { "413150" "123" }
	}
	"1" { "path" "D:\\SteamLibrary" }
}`))
	lf := n.Get("LibraryFolders")
	if lf == nil || len(lf.Kids()) != 2 {
		t.Fatalf("libraryfolders = %+v", lf)
	}
	if got := lf.Get("0").Value("PATH"); got != `C:\Program Files (x86)\Steam` {
		t.Errorf("path = %q", got)
	}
	if got := lf.Get("0", "apps").Value("413150"); got != "123" {
		t.Errorf("apps = %q", got)
	}
	if n.Get("missing", "deeper") != nil || n.Get("missing").Value("x") != "" {
		t.Error("missing keys must be nil-safe")
	}
}

func TestMalformed(t *testing.T) {
	for _, s := range []string{`"a" {`, `}}}}`, `"unterminated`, `{"k" "v"`} {
		_ = Parse(strings.NewReader(s))
	}
}

func FuzzParse(f *testing.F) {
	f.Add(`"AppState" { "appid" "1" "name" "x" }`)
	f.Add("\xef\xbb\xbf// c\n\"k\" \"v\" [$WIN32] p /x { a b } } {")
	f.Fuzz(func(t *testing.T, s string) {
		var check func(n *Node, depth int)
		check = func(n *Node, depth int) {
			if depth > maxNesting {
				t.Fatalf("nested %d levels", depth)
			}
			for k := range n.Values {
				if strings.ToLower(k) != k {
					t.Fatalf("key %q not lower-cased", k)
				}
			}
			for k, c := range n.Children {
				if strings.ToLower(k) != k || c == nil {
					t.Fatalf("child %q: %v", k, c)
				}
				check(c, depth+1)
			}
		}
		check(Parse(strings.NewReader(s)), 1)
	})
}

// FuzzBare checks that a bare (unquoted) value keeps every character.
func FuzzBare(f *testing.F) {
	f.Add("value")
	f.Add("/usr/bin")
	f.Fuzz(func(t *testing.T, v string) {
		if v == "" || strings.ContainsAny(v, " \t\r\n{}\"") || strings.HasPrefix(v, "//") || isConditional(v) {
			return
		}
		if got := Parse(strings.NewReader("k " + v)).Value("k"); got != v {
			t.Fatalf("bare %q parsed as %q", v, got)
		}
	})
}

// FuzzQuoted checks that any key and value survive quoting and escaping.
func FuzzQuoted(f *testing.F) {
	f.Add("Path", `C:\Program Files (x86)\Steam`)
	f.Add("", "")
	f.Add("[$WIN32]", "\"//\"\n{}")
	quote := func(s string) string {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
	}
	f.Fuzz(func(t *testing.T, k, v string) {
		n := Parse(strings.NewReader(`"o" { ` + quote(k) + "\t" + quote(v) + ` } "after" "1"`))
		if got := n.Get("o").Value(k); got != v {
			t.Fatalf("value of %q = %q, want %q", k, got, v)
		}
		if len(n.Get("o").Values) != 1 || n.Value("after") != "1" {
			t.Fatalf("parsed %+v / %+v", n.Get("o"), n)
		}
	})
}

// A shortcuts.vdf as Steam writes it: one shortcut with every value type
// Steam uses there.
func sampleShortcuts() []byte {
	var b []byte
	str := func(k, v string) { b = append(append(append(append(b, BString), k...), 0), append([]byte(v), 0)...) }
	i32 := func(k string, v uint32) {
		b = append(append(append(b, BInt32), k...), 0)
		b = append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	}
	open := func(k string) { b = append(append(append(b, BMap), k...), 0) }
	end := func() { b = append(b, BEnd) }
	open("shortcuts")
	open("0")
	i32("appid", 0x9a3b5c7d)
	str("AppName", "Some Game")
	str("Exe", `"C:\Games\Some Game\game.exe"`)
	str("StartDir", `"C:\Games\Some Game\"`)
	i32("LastPlayTime", 1700000000)
	b = append(append(append(b, BUint64), "big"...), 0)
	b = append(b, 1, 2, 3, 4, 5, 6, 7, 8)
	open("tags")
	str("0", "favorite")
	end()
	end()
	end()
	end()
	return b
}

func TestBinaryRoundTrip(t *testing.T) {
	in := sampleShortcuts()
	root, err := ParseBinary(in)
	if err != nil {
		t.Fatal(err)
	}
	sc := root.Child("shortcuts").Child("0")
	if sc.String("appname") != "Some Game" || sc.Uint("appid") != 0x9a3b5c7d || sc.Child("tags").String("0") != "favorite" {
		t.Fatalf("parsed = %+v", sc)
	}
	out, err := MarshalBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(in) {
		t.Fatalf("round trip changed the file:\n in=%q\nout=%q", in, out)
	}
}

func TestBinaryMalformed(t *testing.T) {
	good := sampleShortcuts()
	if _, err := ParseBinary(good[:len(good)/2]); err == nil {
		t.Error("a truncated file must fail")
	}
	if _, err := ParseBinary([]byte{0x42, 'k', 0}); err == nil {
		t.Error("unknown type must fail")
	}
	deep := []byte{}
	for i := 0; i < 100; i++ {
		deep = append(deep, BMap, 'k', 0)
	}
	if _, err := ParseBinary(deep); err == nil {
		t.Error("deep nesting must fail")
	}
}

func FuzzParseBinary(f *testing.F) {
	f.Add(sampleShortcuts())
	f.Fuzz(func(t *testing.T, b []byte) {
		root, err := ParseBinary(b)
		if err != nil {
			return
		}
		out, err := MarshalBinary(root)
		if err != nil {
			t.Fatal(err)
		}
		again, err := ParseBinary(out)
		if err != nil {
			t.Fatal(err)
		}
		out2, _ := MarshalBinary(again)
		if string(out) != string(out2) {
			t.Fatal("not stable")
		}
	})
}

func TestParseLoginUsers(t *testing.T) {
	n := Parse(strings.NewReader(`// comment
"users"
{
	"76561197960265729"
	{
		"AccountName"		"me"
		"MostRecent"		"1"
		"Path"		"C:\\Games\\x"
	}
}`))
	u := n.Get("Users", "76561197960265729")
	if u.Value("accountname") != "me" || u.Value("MostRecent") != "1" || u.Value("path") != `C:\Games\x` {
		t.Fatalf("bad parse: %+v", u)
	}
}
