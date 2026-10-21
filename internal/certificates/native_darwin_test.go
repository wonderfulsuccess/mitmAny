//go:build darwin && cgo

package certificates

import "testing"

// Malformed input must fail before any keychain write or authorization dialog.
func TestNativeCertificateRejectsMalformedDER(t *testing.T) {
	for _, remove := range []bool{false, true} {
		for _, data := range [][]byte{nil, []byte("not a DER certificate")} {
			if err := nativeCertificate(data, remove); err == nil {
				t.Fatalf("accepted malformed certificate, remove=%v", remove)
			}
		}
	}
}
