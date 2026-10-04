package vdf

import (
	"bytes"
	"sort"
	"strings"
	"testing"
)

// flatten lists a tree as sorted "path=value" lines; objects show as "path/".
func flatten(n *Node) string {
	var lines []string
	var walk func(n *Node, prefix string)
	walk = func(n *Node, prefix string) {
		for k, v := range n.Values {
			lines = append(lines, prefix+k+"="+v)
		}
		for k, c := range n.Children {
			lines = append(lines, prefix+k+"/")
			walk(c, prefix+k+"/")
		}
	}
	walk(n, "")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestParseEdgeCases(t *testing.T) {
	deep := strings.Repeat(`"d" {`, 70) + `"x" "y"` + strings.Repeat("}", 70) + `"k" "v"`
	for _, tt := range []struct {
		name, in string
		want     []string
	}{
		{"empty", ``, nil},
		{"only a comment", `// nothing`, nil},
		{"escapes", `"k" "a\\b\"c\nd\te\x"`, []string{"k=a\\b\"c\nd\te" + "x"}},
		{"bare tokens", `key value`, []string{"key=value"}},
		{"bare value starting with a slash", `p /usr/bin`, []string{"p=/usr/bin"}},
		{"bare value with a slash", `p a/b`, []string{"p=a/b"}},
		{"double slash inside quotes", `"u" "https://x//y"`, []string{"u=https://x//y"}},
		{"comment after a value", "\"k\" \"v\" // note\n\"j\" \"w\"", []string{"j=w", "k=v"}},
		{"comment at end of file", `"k" "v" // note`, []string{"k=v"}},
		{"CRLF line endings", "\"o\"\r\n{\r\n\t\"k\"\t\t\"v\"\r\n}\r\n", []string{"o/", "o/k=v"}},
		{"byte order mark", "\xef\xbb\xbf\"users\" { \"a\" \"1\" }", []string{"users/", "users/a=1"}},
		{"bare key before a brace", `k{ a b }`, []string{"k/", "k/a=b"}},
		{"keys are lower-cased, last one wins", `"Key" "1" "KEY" "2"`, []string{"key=2"}},
		{"empty key and value", `"" ""`, []string{"="}},
		{"empty object", `"o" {}`, []string{"o/"}},
		{"conditional after a value", `"k" "v" [$WIN32] "j" "w"`, []string{"j=w", "k=v"}},
		{"conditional after an object", `"o" { "a" "1" } [$OSX] "b" "2"`, []string{"b=2", "o/", "o/a=1"}},
		{"quoted brackets are a value", `"k" "[$X]" "j" "w"`, []string{"j=w", "k=[$X]"}},
		{"key without value before }", `"o" { "dangling" } "k" "v"`, []string{"k=v", "o/"}},
		{"unterminated string", `"k" "v`, nil},
		{"unterminated escape", `"k" "v\`, nil},
		{"unclosed object", `"o" { "a" "1"`, []string{"o/", "o/a=1"}},
		{"stray closing braces", `} } "k" "v"`, []string{"k=v"}},
		{"object without key", `{ "a" "1" } "k" "v"`, []string{"k=v"}},
		{"deeper than 64 levels", deep, []string{"d/", "k=v"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(strings.NewReader(tt.in))
			// The deep case: check the depth cap, then compare only the top.
			if tt.name == "deeper than 64 levels" {
				n, levels := got, 0
				for n.Get("d") != nil {
					n, levels = n.Get("d"), levels+1
				}
				if levels != maxNesting-1 || n.Value("x") != "" {
					t.Errorf("nested %d levels (want %d), x=%q", levels, maxNesting-1, n.Value("x"))
				}
				got.Children["d"] = newNode()
			}
			if f, want := flatten(got), strings.Join(tt.want, "\n"); f != want {
				t.Errorf("got:\n%s\nwant:\n%s", f, want)
			}
		})
	}
}

func TestNodeNilSafe(t *testing.T) {
	var n *Node
	if n.Get("a") != nil || n.Kids() != nil || n.Value("a") != "" {
		t.Error("nil node must be safe to use")
	}
	if ReadFile("testdata/does-not-exist.vdf") == nil {
		t.Error("ReadFile of a missing file must return an empty node")
	}
	if n := Parse(strings.NewReader(`"a" { "b" "1" }`)); n.Get() != n {
		t.Error("Get with no keys must return the node itself")
	}
}

// bin builds binary KeyValues: type byte, key, NUL, then the payload.
func bin(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func entry(t byte, key string, payload ...byte) []byte {
	return append(append(append([]byte{t}, key...), 0), payload...)
}

func TestParseBinaryEdgeCases(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      []byte
		want    string // dumpBinary output
		wantErr string
	}{
		{"empty", nil, "", ""},
		{"only the end marker", []byte{BEnd}, "", ""},
		{"data after the end marker is ignored", bin([]byte{BEnd}, entry(BString, "k", 'v', 0)), "", ""},
		{"no end marker at top level", entry(BString, "k", 'v', 0), "\"k\" string \"v\"\n", ""},
		{"empty key and string", entry(BString, "", 0), "\"\" string \"\"\n", ""},
		{"int32 is little-endian", entry(BInt32, "n", 1, 2, 3, 4), "\"n\" int32 67305985\n", ""},
		{"float kept raw", entry(BFloat, "f", 0, 0, 0x80, 0x3f), "\"f\" type 0x03 raw 0000803f\n", ""},
		{"pointer kept raw", entry(BPtr, "p", 1, 2, 3, 4), "\"p\" type 0x04 raw 01020304\n", ""},
		{"colour kept raw", entry(BColor, "c", 255, 0, 0, 255), "\"c\" type 0x06 raw ff0000ff\n", ""},
		{"uint64 kept raw", entry(BUint64, "u", 1, 2, 3, 4, 5, 6, 7, 8), "\"u\" type 0x07 raw 0102030405060708\n", ""},
		{"int64 kept raw", entry(BInt64, "i", 8, 7, 6, 5, 4, 3, 2, 1), "\"i\" type 0x0a raw 0807060504030201\n", ""},
		{"empty map", bin(entry(BMap, "m"), []byte{BEnd}), "\"m\" {\n}\n", ""},
		{"unclosed map", entry(BMap, "m"), "", "unexpected end"},
		{"truncated key", []byte{BString, 'k'}, "", "unexpected end"},
		{"truncated string", entry(BString, "k", 'v'), "", "unexpected end"},
		{"truncated int32", entry(BInt32, "n", 1, 2), "", "unexpected end"},
		{"truncated float", entry(BFloat, "f", 1), "", "unexpected end"},
		{"truncated uint64", entry(BUint64, "u", 1, 2, 3, 4), "", "unexpected end"},
		{"unknown type", entry(0x05, "w"), "", "unknown value type 0x05"},
		{"string too long", entry(BString, "k", bytes.Repeat([]byte{'a'}, 1<<20+2)...), "", "too long"},
		{"nested too deeply", bytes.Repeat(entry(BMap, "m"), maxDepth+2), "", "nested too deeply"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, err := ParseBinary(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := dumpBinary(root); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestParseBinaryTooLarge(t *testing.T) {
	if _, err := ParseBinary(make([]byte, maxFile+1)); err == nil {
		t.Error("a file over the size cap must fail")
	}
}

func TestBNodeAccessors(t *testing.T) {
	root, err := ParseBinary(bin(entry(BString, "Name", 'x', 0), entry(BInt32, "ID", 7, 0, 0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		got  any
		want any
	}{
		{"string, any case", root.String("name"), "x"},
		{"int, any case", root.Uint("id"), uint32(7)},
		{"string of an int", root.String("id"), ""},
		{"int of a string", root.Uint("name"), uint32(0)},
		{"missing string", root.String("nope"), ""},
		{"missing child", root.Child("nope") == nil, true},
		{"nil node", (*BNode)(nil).String("name"), ""},
	} {
		if tt.got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestMarshalBinaryErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		kid     *BNode
		wantErr string
	}{
		{"NUL in key", &BNode{Key: "a\x00b", Type: BString}, "key contains NUL"},
		{"NUL in value", &BNode{Key: "k", Type: BString, Str: "a\x00b"}, "value contains NUL"},
		{"short raw", &BNode{Key: "f", Type: BFloat, Raw: []byte{1}}, "has 1 bytes, want 4"},
		{"long raw", &BNode{Key: "u", Type: BUint64, Raw: make([]byte, 9)}, "has 9 bytes, want 8"},
		{"unknown type", &BNode{Key: "w", Type: 0x05}, "unknown value type"},
		{"error inside a map", &BNode{Key: "m", Type: BMap, Kids: []*BNode{{Key: "x\x00", Type: BString}}}, "key contains NUL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MarshalBinary(&BNode{Type: BMap, Kids: []*BNode{tt.kid}})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestMarshalBinaryEdits(t *testing.T) {
	root, err := ParseBinary(sampleShortcuts())
	if err != nil {
		t.Fatal(err)
	}
	sc := root.Child("shortcuts")
	sc.Kids = append(sc.Kids, &BNode{Key: "1", Type: BMap, Kids: []*BNode{
		{Key: "AppName", Type: BString, Str: "Added"},
		{Key: "appid", Type: BInt32, Int: 42},
	}})
	out, err := MarshalBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseBinary(out)
	if err != nil {
		t.Fatal(err)
	}
	added := again.Child("shortcuts").Child("1")
	if added.String("appname") != "Added" || added.Uint("appid") != 42 || again.Child("shortcuts").Child("0").String("AppName") != "Some Game" {
		t.Errorf("after an edit: %s", dumpBinary(again))
	}
}
