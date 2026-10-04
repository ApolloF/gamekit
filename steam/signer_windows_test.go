package steam

import (
	"crypto/x509/pkix"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The subjects on real Valve certificates, as found on Steam installs.
var (
	valveCorp = pkix.Name{CommonName: "Valve Corp.", Organization: []string{"Valve Corp."},
		Locality: []string{"Bellevue"}, Province: []string{"Washington"}, Country: []string{"US"}} // Steam client, 2024
	valveShort = pkix.Name{CommonName: "Valve", Organization: []string{"Valve"},
		Locality: []string{"Bellevue"}, Province: []string{"WA"}, Country: []string{"US"}} // steam_api64.dll, 2018
)

func TestIsValve(t *testing.T) {
	with := func(f func(*pkix.Name)) pkix.Name {
		n := valveCorp
		f(&n)
		return n
	}
	for _, tc := range []struct {
		desc string
		name pkix.Name
		want bool
	}{
		{"Valve Corp.", valveCorp, true},
		{"Valve", valveShort, true},
		{"case and spaces", with(func(n *pkix.Name) { n.CommonName, n.Organization = " valve corp. ", []string{"VALVE"} }), true},
		{"other signer", pkix.Name{CommonName: "Cheap Cert Ltd", Organization: []string{"Cheap Cert Ltd"}, Country: []string{"US"}}, false},
		{"lookalike CN", with(func(n *pkix.Name) { n.CommonName = "Valve Corp. Fake" }), false},
		{"Valve CN, other O", with(func(n *pkix.Name) { n.Organization = []string{"Goldberg"} }), false},
		{"Valve CN, extra O", with(func(n *pkix.Name) { n.Organization = []string{"Valve", "Goldberg"} }), false},
		{"Valve O, other CN", with(func(n *pkix.Name) { n.CommonName = "Goldberg" }), false},
		{"no O", with(func(n *pkix.Name) { n.Organization = nil }), false},
		{"not US", with(func(n *pkix.Name) { n.Country = []string{"RU"} }), false},
		{"no country", with(func(n *pkix.Name) { n.Country = nil }), false},
		{"empty", pkix.Name{}, false},
	} {
		if got := isValve(tc.name); got != tc.want {
			t.Errorf("%s: isValve(%v) = %v, want %v", tc.desc, tc.name, got, tc.want)
		}
	}
}

func TestSignedBy(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	subject := func(n pkix.Name, ok bool) func(string) (pkix.Name, bool) {
		return func(string) (pkix.Name, bool) { return n, ok }
	}
	for _, tc := range []struct {
		desc    string
		trusted func(string) bool
		subject func(string) (pkix.Name, bool)
		want    bool
	}{
		{"Valve Corp.", yes, subject(valveCorp, true), true},
		{"Valve", yes, subject(valveShort, true), true},
		{"other valid signer", yes, subject(pkix.Name{CommonName: "Microsoft Corporation", Organization: []string{"Microsoft Corporation"}, Country: []string{"US"}}, true), false},
		{"no signer", yes, subject(pkix.Name{}, false), false},
		{"Valve but not trusted", no, subject(valveCorp, true), false},
		{"Valve but not trusted, short name", no, subject(valveShort, true), false},
	} {
		if got := signedBy("x.dll", tc.trusted, tc.subject); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.desc, got, tc.want)
		}
	}
}

func TestSignerSubjectUnsigned(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.dll")
	if err := os.WriteFile(p, []byte("MZ not signed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, ok := signerSubject(p); ok {
		t.Errorf("signerSubject(unsigned) = %v", s)
	}
}

// A signed system binary has a signer, but it isn't Valve, so Signed must
// refuse it.
func TestSignedRejectsNonValveSigner(t *testing.T) {
	p := filepath.Join(os.Getenv("SystemRoot"), "System32", "kernel32.dll")
	if _, err := os.Stat(p); err != nil {
		t.Skip("no kernel32.dll")
	}
	s, ok := signerSubject(p)
	if !ok {
		t.Skip("kernel32.dll is catalog-signed here")
	}
	if Signed(p) {
		t.Errorf("Signed(kernel32.dll) = true, signer %v", s)
	}
}

// Every Valve-signed Steam binary on this PC must pass, whatever name form
// its certificate uses. Skipped where Steam isn't installed (CI).
func TestSignedAcceptsLocalValveBinaries(t *testing.T) {
	d := Dir()
	if d == "" {
		t.Skip("Steam isn't installed")
	}
	files := []string{filepath.Join(d, "steam.exe")}
	for _, lib := range Libraries(d) {
		files = append(files, steamAPIDLLs(filepath.Join(lib, "steamapps", "common"))...)
	}
	checked := 0
	for _, p := range files {
		s, ok := signerSubject(p)
		if !ok || !containsFold(s.Organization, "valve") {
			continue // unsigned or not Valve's: nothing to assert
		}
		checked++
		if !Signed(p) {
			t.Errorf("Signed(%s) = false, signer %v", p, s)
		}
	}
	if checked == 0 {
		t.Skip("no Valve-signed binary on this PC")
	}
	t.Logf("%d Valve-signed binaries accepted", checked)
}

func steamAPIDLLs(root string) []string {
	var out []string
	depth0 := strings.Count(filepath.Clean(root), string(filepath.Separator))
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.Count(p, string(filepath.Separator))-depth0 > tamperDepth {
			return filepath.SkipDir
		}
		if n := strings.ToLower(d.Name()); !d.IsDir() && (n == "steam_api.dll" || n == "steam_api64.dll") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func containsFold(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(strings.ToLower(s), sub) {
			return true
		}
	}
	return false
}
