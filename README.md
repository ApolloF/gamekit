# gamekit

Go packages for finding PC games on Windows, shared by [Seaglass](https://github.com/ApolloF/Seaglass) and [Syncer](https://github.com/ApolloF/syncer).

| Package | What it does |
|---|---|
| `vdf` | Valve's KeyValues formats: text (`libraryfolders.vdf`, `appmanifest_*.acf`, `loginusers.vdf`, `localconfig.vdf`) and binary (`shortcuts.vdf`), read and written without disturbing what Steam put there |
| `steam` | Where Steam is, its library folders and installed games, which account is used on this PC, whether a game is running, and whether a game's folder was cracked or runs on a Steam emulator (marker files, Goldberg's `steam_settings`, an unsigned `steam_api` DLL) |
| `ludusavi` | The community [Ludusavi manifest](https://github.com/mtkennerly/ludusavi-manifest): ~50,000 games with their Steam and GOG ids, install folder names, Windows save locations (including store folders such as Ubisoft Connect's `savegames`), and Steam Cloud and Ubisoft Connect cloud support. Parses the full file in under 100 ms |

Every parser treats its input as untrusted: sizes are capped, nesting is bounded, and the parsers are fuzz-tested.

```go
import (
	"github.com/ApolloF/gamekit/ludusavi"
	"github.com/ApolloF/gamekit/steam"
)

root := steam.Dir()
for id, app := range steam.Apps(root) {
	if app.Installed {
		fmt.Println(id, app.Name, app.Dir, steam.Tampered(app.Dir, steam.Signed))
	}
}
accounts := steam.Accounts(root, steam.HostInfo())
```

The packages only read; nothing is downloaded or changed on their own. Writing a `shortcuts.vdf` back is up to the caller (Steam must be closed).

## Develop

```bash
go vet ./... && GOOS=linux go vet ./...
go test -race ./...
go test ./... -update                           # rewrite the golden files in */testdata after an intended change
go test ./vdf -run '^$' -fuzz '^FuzzParse$' -fuzztime 5m   # also FuzzQuoted, FuzzBare, FuzzParseBinary; ./ludusavi FuzzParse
LUDUSAVI_MANIFEST=manifest.yaml go test -run ParseReal -v ./ludusavi
```

Inputs a fuzz target found a bug with are kept in `*/testdata/fuzz` and run by every `go test`.

## License

[MIT](LICENSE)
