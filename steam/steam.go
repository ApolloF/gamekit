// Package steam reads a Steam installation: where Steam is, its library
// folders and installed games, which account is used on this PC, and
// whether a game's folder was cracked or runs on a Steam emulator.
package steam

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ApolloF/gamekit/vdf"
)

// IDBase is the steamID64 of account id 0; userdata folders are named by the
// 32-bit account id (steamID64 - IDBase).
const IDBase = 76561197960265728

func readVDF(p string) *vdf.Node { return vdf.ReadFile(p) }

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func clean(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// within reports whether child is parent or lies below it (case-insensitive).
func within(parent, child string) bool {
	parent, child = strings.ToLower(filepath.Clean(parent)), strings.ToLower(filepath.Clean(child))
	if parent == child {
		return true
	}
	if !strings.HasSuffix(parent, `\`) && !strings.HasSuffix(parent, "/") {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(child, parent)
}
