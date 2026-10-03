package vdf_test

import (
	"fmt"
	"strings"

	"github.com/ApolloF/gamekit/vdf"
)

func ExampleParse() {
	n := vdf.Parse(strings.NewReader(`
"AppState"
{
	"appid"		"413150"
	"name"		"Stardew Valley"
	"UserConfig" { "language" "english" }
}`))
	app := n.Get("AppState")
	fmt.Println(app.Value("appid"), app.Value("Name"))
	fmt.Println(app.Get("userconfig").Value("language"))
	fmt.Println(n.Get("AppState", "missing").Value("x") == "")
	// Output:
	// 413150 Stardew Valley
	// english
	// true
}

func ExampleNode_Kids() {
	n := vdf.Parse(strings.NewReader(`"libraryfolders" {
		"0" { "path" "C:\\Program Files (x86)\\Steam" }
	}`))
	for _, lib := range n.Get("libraryfolders").Kids() {
		fmt.Println(lib.Value("path"))
	}
	// Output: C:\Program Files (x86)\Steam
}

func ExampleParseBinary() {
	in := []byte("\x00shortcuts\x00" +
		"\x000\x00" +
		"\x02appid\x00\x7d\x5c\x3b\x9a" +
		"\x01AppName\x00Some Game\x00" +
		"\x08" + // end of shortcut 0
		"\x08" + // end of shortcuts
		"\x08") // end of document
	root, err := vdf.ParseBinary(in)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, sc := range root.Child("shortcuts").Kids {
		fmt.Printf("%s %#x\n", sc.String("appname"), sc.Uint("appid"))
	}
	// Output: Some Game 0x9a3b5c7d
}

func ExampleMarshalBinary() {
	root := &vdf.BNode{Type: vdf.BMap, Kids: []*vdf.BNode{
		{Key: "shortcuts", Type: vdf.BMap, Kids: []*vdf.BNode{
			{Key: "0", Type: vdf.BMap, Kids: []*vdf.BNode{
				{Key: "AppName", Type: vdf.BString, Str: "Some Game"},
				{Key: "IsHidden", Type: vdf.BInt32, Int: 0},
			}},
		}},
	}}
	b, err := vdf.MarshalBinary(root)
	if err != nil {
		fmt.Println(err)
		return
	}
	back, _ := vdf.ParseBinary(b)
	fmt.Println(len(b), back.Child("shortcuts").Child("0").String("AppName"))
	// Output: 50 Some Game
}
