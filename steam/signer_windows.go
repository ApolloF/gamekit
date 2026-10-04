package steam

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"slices"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	crypt32              = windows.NewLazySystemDLL("crypt32.dll")
	procCryptMsgGetParam = crypt32.NewProc("CryptMsgGetParam")
	procCryptMsgClose    = crypt32.NewProc("CryptMsgClose")
)

const (
	cmsgSignerInfoParam = 6 // CMSG_SIGNER_INFO_PARAM
	encoding            = windows.X509_ASN_ENCODING | windows.PKCS_7_ASN_ENCODING
)

// signerInfo is the start of CMSG_SIGNER_INFO: enough to find the signer's
// certificate by issuer and serial number.
type signerInfo struct {
	Version      uint32
	Issuer       windows.CertNameBlob
	SerialNumber windows.CryptIntegerBlob
}

// signerSubject returns the subject of the certificate that signed the
// file's embedded Authenticode signature; false if there is none.
func signerSubject(path string) (pkix.Name, bool) {
	p16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return pkix.Name{}, false
	}
	var store, msg windows.Handle
	err = windows.CryptQueryObject(windows.CERT_QUERY_OBJECT_FILE, unsafe.Pointer(p16),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED, windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0, nil, nil, nil, &store, &msg, nil)
	if err != nil {
		return pkix.Name{}, false
	}
	defer windows.CertCloseStore(store, 0)
	defer procCryptMsgClose.Call(uintptr(msg))

	var size uint32
	if r, _, _ := procCryptMsgGetParam.Call(uintptr(msg), cmsgSignerInfoParam, 0, 0, uintptr(unsafe.Pointer(&size))); r == 0 || size < uint32(unsafe.Sizeof(signerInfo{})) {
		return pkix.Name{}, false
	}
	buf := make([]byte, size)
	if r, _, _ := procCryptMsgGetParam.Call(uintptr(msg), cmsgSignerInfoParam, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return pkix.Name{}, false
	}
	si := (*signerInfo)(unsafe.Pointer(&buf[0]))
	want := windows.CertInfo{Issuer: si.Issuer, SerialNumber: si.SerialNumber}
	cert, err := windows.CertFindCertificateInStore(store, encoding, 0, windows.CERT_FIND_SUBJECT_CERT, unsafe.Pointer(&want), nil)
	if err != nil {
		return pkix.Name{}, false
	}
	defer windows.CertFreeCertificateContext(cert)

	// Clone: the parsed certificate points into its input, which is freed on return.
	c, err := x509.ParseCertificate(slices.Clone(unsafe.Slice(cert.EncodedCert, cert.Length)))
	if err != nil {
		return pkix.Name{}, false
	}
	return c.Subject, true
}
