package steam

import (
	"crypto/x509/pkix"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const steamKey = `Software\Valve\Steam`

// HostInfo reads what Steam keeps in the registry.
func HostInfo() Host {
	h := Host{Running: Running, Signed: Signed}
	if k, err := registry.OpenKey(registry.CURRENT_USER, steamKey+`\ActiveProcess`, registry.QUERY_VALUE); err == nil {
		if v, _, err := k.GetIntegerValue("ActiveUser"); err == nil {
			h.ActiveUser = uint32(v)
		}
		k.Close()
	}
	if k, err := registry.OpenKey(registry.CURRENT_USER, steamKey, registry.QUERY_VALUE); err == nil {
		h.AutoLogin, _, _ = k.GetStringValue("AutoLoginUser")
		k.Close()
	}
	return h
}

// Running reports whether Steam says the game is running right now.
func Running(appID int) bool {
	if k, err := registry.OpenKey(registry.CURRENT_USER, steamKey, registry.QUERY_VALUE); err == nil {
		v, _, err := k.GetIntegerValue("RunningAppID")
		k.Close()
		if err == nil && int(v) == appID {
			return true
		}
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, steamKey+`\Apps\`+strconv.Itoa(appID), registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("Running")
	return err == nil && v != 0
}

var sigCache struct {
	sync.Mutex
	m map[string]bool
}

// valveNames are the names on Valve's Authenticode certificates, both as CN
// and O: "Valve Corp." on the Steam client and newer steam_api DLLs (DigiCert
// Trusted G4 Code Signing CA, 2024), "Valve" on older steam_api DLLs that
// games still ship (DigiCert SHA2 Assured ID Code Signing CA, 2018).
var valveNames = []string{"Valve Corp.", "Valve"}

// Signed reports whether the file carries a valid Authenticode signature
// made by Valve. Steam's steam_api DLLs are signed by Valve; emulator DLLs
// aren't, or are signed by someone else, and a patched one fails the hash
// check. No revocation or network lookups.
func Signed(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	key := path + "|" + strconv.FormatInt(fi.Size(), 10) + "|" + fi.ModTime().Format(time.RFC3339Nano)
	sigCache.Lock()
	v, ok := sigCache.m[key]
	sigCache.Unlock()
	if ok {
		return v
	}
	v = signedBy(path, trusted, signerSubject)
	sigCache.Lock()
	if sigCache.m == nil {
		sigCache.m = map[string]bool{}
	}
	sigCache.m[key] = v
	sigCache.Unlock()
	return v
}

// signedBy reports whether path has a valid signature (trusted) whose signer
// certificate (subject) belongs to Valve.
func signedBy(path string, trusted func(string) bool, subject func(string) (pkix.Name, bool)) bool {
	if !trusted(path) {
		return false
	}
	s, ok := subject(path)
	return ok && isValve(s)
}

// isValve reports whether a certificate subject is Valve's: CN and every O
// are Valve names and the country is US. Requiring O as well as CN keeps out
// a certificate that merely calls itself Valve in one field.
func isValve(s pkix.Name) bool {
	if !valveName(s.CommonName) || len(s.Organization) == 0 || !slices.Equal(s.Country, []string{"US"}) {
		return false
	}
	for _, o := range s.Organization {
		if !valveName(o) {
			return false
		}
	}
	return true
}

func valveName(s string) bool {
	s = strings.TrimSpace(s)
	return slices.ContainsFunc(valveNames, func(v string) bool { return strings.EqualFold(s, v) })
}

// trusted reports whether WinVerifyTrust accepts the file's signature.
func trusted(path string) bool {
	p16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	data := &windows.WinTrustData{
		Size:             uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:         windows.WTD_UI_NONE,
		RevocationChecks: windows.WTD_REVOKE_NONE,
		UnionChoice:      windows.WTD_CHOICE_FILE,
		StateAction:      windows.WTD_STATEACTION_VERIFY,
		ProvFlags:        windows.WTD_CACHE_ONLY_URL_RETRIEVAL,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&windows.WinTrustFileInfo{
			Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
			FilePath: p16,
		}),
	}
	err = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	return err == nil
}
