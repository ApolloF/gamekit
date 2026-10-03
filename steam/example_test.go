package steam_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ApolloF/gamekit/steam"
)

// List the installed Steam games of this PC.
func ExampleApps() {
	root := steam.Dir()
	if root == "" {
		return // Steam isn't installed
	}
	apps := steam.Apps(root)
	ids := make([]int, 0, len(apps))
	for id := range apps {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if a := apps[id]; a.Installed {
			fmt.Println(a.ID, a.Name, a.Dir)
		}
	}
}

// Find the userdata folder of the account Steam uses on this PC, where
// Steam Cloud keeps a game's saves.
func ExampleAccounts() {
	root := steam.Dir()
	for _, acc := range steam.Accounts(root, steam.HostInfo()) {
		fmt.Println(filepath.Join(root, "userdata", acc, "413150", "remote"))
	}
}

func ExampleTampered() {
	dir, err := os.MkdirTemp("", "game")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(dir)
	_ = os.WriteFile(filepath.Join(dir, "steam_api64.dll"), []byte("MZ"), 0o644)

	valveSigned := func(string) bool { return true } // steam.Signed in real use
	fmt.Printf("%q\n", steam.Tampered(dir, valveSigned))

	_ = os.WriteFile(filepath.Join(dir, "steam_emu.ini"), []byte("[Settings]"), 0o644)
	fmt.Printf("%q\n", steam.Tampered(dir, valveSigned))
	// Output:
	// ""
	// "steam_emu.ini"
}
