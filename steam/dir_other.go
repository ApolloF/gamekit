//go:build !windows

package steam

// Dir returns Steam's install folder; only Steam on Windows is supported.
func Dir() string { return "" }

// HostInfo returns what the registry says about Steam; nothing off Windows.
func HostInfo() Host { return Host{} }

// Signed reports whether a file carries a valid Authenticode signature;
// never off Windows.
func Signed(string) bool { return false }

// Running reports whether Steam says a game runs; never off Windows.
func Running(int) bool { return false }
