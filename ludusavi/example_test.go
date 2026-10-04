package ludusavi_test

import (
	"fmt"
	"strings"

	"github.com/ApolloF/gamekit/ludusavi"
)

func ExampleParse() {
	const manifest = `---
"Assassin's Creed Odyssey":
  cloud:
    uplay: true
  files:
    "<root>/savegames/<storeUserId>/5059":
      tags:
        - save
      when:
        - store: uplay
  installDir:
    Assassins Creed Odyssey: {}
  steam:
    id: 812140
Stardew Valley:
  cloud:
    steam: true
  files:
    "<winAppData>/StardewValley/Saves":
      tags:
        - save
      when:
        - os: windows
  steam:
    id: 413150
Stardew:
  alias: Stardew Valley
`
	games, err := ludusavi.Parse(strings.NewReader(manifest))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, g := range games {
		fmt.Printf("%s (Steam %d) aliases=%q saves=%q steam cloud=%v\n", g.Name, g.SteamID, g.Aliases, g.Saves, g.SteamCloud)
		for _, r := range g.RootSaves {
			fmt.Printf("  in the %q folder: %s\n", r.Stores, r.Path)
		}
	}
	// Output:
	// Assassin's Creed Odyssey (Steam 812140) aliases=[] saves=[] steam cloud=false
	//   in the ["uplay"] folder: <root>/savegames/<storeUserId>/5059
	// Stardew Valley (Steam 413150) aliases=["Stardew"] saves=["<winAppData>/StardewValley/Saves"] steam cloud=true
}
