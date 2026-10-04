package ludusavi

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRejectsTraversal(t *testing.T) {
	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"<winAppData>/Game/Saves", true},
		{"<winDocuments>/My Games/G/<storeUserId>", true},
		{"<winAppData>/Game/a..b", true},
		{"<winAppData>/..hidden/x", true},
		{"<winAppData>/../../../../Windows/System32", false},
		{"<winAppData>/Game/../../../Windows", false},
		{`<winAppData>/Game\..\..\Windows`, false},
		{"<winAppData>/Game/..", false},
		{"<winAppData>C:/Windows", false},
		{"<winAppData>//server/share", false},
		{`<winAppData>\Game`, false},
		{"<winAppData>/C:/Windows", false},
		{"<root>/Saves", true},
		{"<root>/../../../Users/Public", false},
		{"<root>/Saves/../../x", false},
		{"<root>//server/share", false},
	} {
		m := "G:\n  files:\n    \"" + tc.path + "\":\n      tags:\n        - save\n"
		es, err := Parse(strings.NewReader(m))
		if err != nil || len(es) != 1 {
			t.Fatalf("%q: %v %+v", tc.path, err, es)
		}
		var got []string
		got = append(got, es[0].Saves...)
		for _, r := range es[0].RootSaves {
			got = append(got, r.Path)
		}
		want := []string(nil)
		if tc.ok {
			want = []string{tc.path}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %q, want %q", tc.path, got, want)
		}
	}
}
