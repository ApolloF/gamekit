package steam

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// App is a game Steam installed: it has an appmanifest_<id>.acf in a library.
// Copies installed outside Steam (a standalone install, a backup) have none.
type App struct {
	ID        int
	Name      string
	Dir       string // install folder; "" if the manifest's installdir is unsafe
	Installed bool   // installed and playable through Steam (possibly waiting for an update)
	Size      int64  // bytes on disk, as Steam recorded it
}

// StateFlags bits of an appmanifest (EAppState).
const (
	stateUpdateRequired = 2
	stateFullyInstalled = 4
	stateUpdateRunning  = 256
	stateUpdatePaused   = 512
	stateUpdateStarted  = 1024
	stateUninstalling   = 2048
)

// Libraries returns Steam's own folder plus every library folder listed in
// its libraryfolders.vdf, in the current format ("1" { "path" "D:\\Lib" })
// or the one Steam used before 2021 ("1" "D:\\Lib").
func Libraries(root string) []string {
	if !filepath.IsAbs(root) {
		return nil
	}
	libs := []string{filepath.Clean(root)}
	seen := map[string]bool{strings.ToLower(libs[0]): true}
	var extra []string
	lf := readVDF(filepath.Join(root, "steamapps", "libraryfolders.vdf")).Get("libraryfolders")
	var paths []string
	for _, lib := range lf.Kids() {
		paths = append(paths, lib.Value("path"))
	}
	if lf != nil {
		for k, v := range lf.Values {
			if _, err := strconv.ParseUint(k, 10, 32); err == nil {
				paths = append(paths, v)
			}
		}
	}
	sort.Strings(paths) // the vdf's maps have no order; keep the result stable
	for _, p := range paths {
		dir := filepath.Clean(p)
		if key := strings.ToLower(dir); filepath.IsAbs(dir) && !seen[key] {
			extra = append(extra, dir)
			seen[key] = true
		}
	}
	sort.Strings(extra)
	return append(libs, extra...)
}

// LibraryApps reads the app manifests of one Steam library.
func LibraryApps(lib string) []App {
	files, _ := filepath.Glob(filepath.Join(lib, "steamapps", "appmanifest_*.acf"))
	var out []App
	for _, file := range files {
		st := readVDF(file).Get("AppState")
		id, err := strconv.Atoi(st.Value("appid"))
		if err != nil || id <= 0 {
			continue
		}
		app := App{ID: id, Name: st.Value("name")}
		app.Size, _ = strconv.ParseInt(st.Value("SizeOnDisk"), 10, 64)
		common := filepath.Join(lib, "steamapps", "common")
		installDir := st.Value("installdir")
		if dir := filepath.Join(common, installDir); filepath.IsLocal(installDir) && !strings.EqualFold(dir, common) && within(common, dir) {
			app.Dir = dir
		}
		flags, _ := strconv.Atoi(st.Value("StateFlags"))
		playable := flags&stateFullyInstalled != 0 ||
			flags&(stateUpdateRequired|stateUpdateRunning|stateUpdatePaused|stateUpdateStarted) != 0
		app.Installed = playable && flags&stateUninstalling == 0 && app.Dir != "" && isDir(app.Dir)
		out = append(out, app)
	}
	return out
}

// Apps returns every game Steam knows as installed, across all libraries.
func Apps(root string) map[int]App {
	m := map[int]App{}
	for _, lib := range Libraries(root) {
		for _, a := range LibraryApps(lib) {
			if old, ok := m[a.ID]; !ok || (!old.Installed && a.Installed) {
				m[a.ID] = a
			}
		}
	}
	return m
}
