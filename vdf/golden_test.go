package vdf

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// golden compares got with the file at p, or rewrites it with -update.
func golden(t *testing.T, p string, got []byte) {
	t.Helper()
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
		t.Errorf("%s differs from the parsed result (go test -update rewrites it):\n got:\n%s\nwant:\n%s", p, got, want)
	}
}

func TestParseGolden(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "text", "*.vdf"))
	acf, _ := filepath.Glob(filepath.Join("testdata", "text", "*.acf"))
	files = append(files, acf...)
	if len(files) == 0 {
		t.Fatal("no testdata")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			got, err := json.MarshalIndent(ReadFile(f), "", "\t")
			if err != nil {
				t.Fatal(err)
			}
			golden(t, f+".golden", append(got, '\n'))
		})
	}
}

func TestParseBinaryGolden(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "binary", "*.vdf"))
	if len(files) == 0 {
		t.Fatal("no testdata")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			in, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			root, err := ParseBinary(in)
			if err != nil {
				t.Fatal(err)
			}
			golden(t, f+".golden", []byte(dumpBinary(root)))
			out, err := MarshalBinary(root)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != string(in) {
				t.Error("writing the file back changed it")
			}
		})
	}
}

// dumpBinary renders a binary KeyValues tree one entry per line, with types.
func dumpBinary(n *BNode) string {
	var sb strings.Builder
	var walk func(n *BNode, indent string)
	walk = func(n *BNode, indent string) {
		for _, k := range n.Kids {
			key := strconv.Quote(k.Key)
			switch k.Type {
			case BMap:
				fmt.Fprintf(&sb, "%s%s {\n", indent, key)
				walk(k, indent+"\t")
				fmt.Fprintf(&sb, "%s}\n", indent)
			case BString:
				fmt.Fprintf(&sb, "%s%s string %q\n", indent, key, k.Str)
			case BInt32:
				fmt.Fprintf(&sb, "%s%s int32 %d\n", indent, key, k.Int)
			default:
				fmt.Fprintf(&sb, "%s%s type 0x%02x raw %x\n", indent, key, k.Type, k.Raw)
			}
		}
	}
	walk(n, "")
	return sb.String()
}
