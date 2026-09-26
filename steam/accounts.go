package steam

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Host is what Steam's files don't say: which account is signed in right
// now and what's running. HostInfo fills it in from the registry and
// Windows; tests pass their own.
type Host struct {
	ActiveUser uint32                 // account id Steam is signed in with (0 = Steam closed / unknown)
	AutoLogin  string                 // account name Steam signs in with automatically
	Running    func(appID int) bool   // Steam reports the game running
	Signed     func(path string) bool // the file carries a valid Authenticode signature
}

// Accounts returns the userdata folder names (32-bit account ids) of the
// account Steam uses on this PC: the one signed in now, else the one that
// signs in automatically, else the last one used. Current Steam builds no
// longer mark it "MostRecent" in loginusers.vdf, so the registry is asked
// first. All known accounts if it can't be told.
func Accounts(dir string, h Host) []string {
	type user struct {
		acc, name string
		recent    bool
		ts        int64
	}
	var users []user
	for id64, u := range readVDF(filepath.Join(dir, "config", "loginusers.vdf")).Get("users").Kids() {
		n, err := strconv.ParseUint(id64, 10, 64)
		if err != nil || n <= IDBase {
			continue
		}
		acc := strconv.FormatUint(n-IDBase, 10)
		if !isDir(filepath.Join(dir, "userdata", acc)) {
			continue
		}
		ts, _ := strconv.ParseInt(u.Value("Timestamp"), 10, 64)
		users = append(users, user{acc, u.Value("AccountName"), u.Value("MostRecent") == "1", ts})
	}
	sort.Slice(users, func(i, j int) bool { return users[i].acc < users[j].acc })

	// Signed in right now.
	if h.ActiveUser != 0 {
		if acc := strconv.FormatUint(uint64(h.ActiveUser), 10); isDir(filepath.Join(dir, "userdata", acc)) {
			return []string{acc}
		}
	}
	// Signs in automatically when Steam starts.
	if h.AutoLogin != "" {
		for _, u := range users {
			if strings.EqualFold(u.name, h.AutoLogin) {
				return []string{u.acc}
			}
		}
	}
	// Older Steam builds mark the last account.
	var all []string
	for _, u := range users {
		if u.recent {
			return []string{u.acc}
		}
		all = append(all, u.acc)
	}
	// The account that signed in last.
	best := -1
	for i, u := range users {
		if u.ts > 0 && (best < 0 || u.ts > users[best].ts) {
			best = i
		}
	}
	if best >= 0 {
		return []string{users[best].acc}
	}
	if len(all) > 0 {
		return all
	}
	es, _ := os.ReadDir(filepath.Join(dir, "userdata"))
	for _, e := range es {
		if _, err := strconv.Atoi(e.Name()); err == nil && e.IsDir() && e.Name() != "0" {
			all = append(all, e.Name())
		}
	}
	return all
}
