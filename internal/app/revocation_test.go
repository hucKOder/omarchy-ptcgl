package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sassoftware/relic/v8/lib/certloader"
	"github.com/sassoftware/relic/v8/lib/pkcs9"
)

func crlDER(t *testing.T, value any) []byte {
	t.Helper()
	data, err := asn1.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func crlScope(t *testing.T, point string) []byte {
	t.Helper()
	uri := crlDER(t, asn1.RawValue{Class: 2, Tag: 6, Bytes: []byte(point)})
	name := crlDER(t, asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: uri})
	field := crlDER(t, asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: name})
	return crlDER(t, asn1.RawValue{Tag: asn1.TagSequence, IsCompound: true, Bytes: field})
}

func testCRL(t *testing.T, ca *x509.Certificate, key *rsa.PrivateKey, template *x509.RevocationList) []byte {
	t.Helper()
	data, err := x509.CreateRevocationList(rand.Reader, template, ca, key)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSignedRevocationLists(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"},
		NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	ca := testCertificate(t, key, caTemplate, caTemplate)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), CRLDistributionPoints: []string{"http://example.test/list.crl"}}
	for _, mode := range []string{"valid", "revoked", "stale", "future", "tampered", "other issuer", "matching scope", "other scope", "partial reasons", "indirect", "CA only", "delta", "unknown critical"} {
		t.Run(mode, func(t *testing.T) {
			list := &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: now.Add(-time.Hour), NextUpdate: now.Add(time.Hour)}
			scope := func(data []byte) {
				list.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 28}, Critical: true, Value: data}}
			}
			switch mode {
			case "revoked":
				list.RevokedCertificateEntries = []x509.RevocationListEntry{{SerialNumber: leaf.SerialNumber, RevocationTime: now.Add(-time.Hour)}}
			case "stale":
				list.NextUpdate = now.Add(-time.Minute)
			case "future":
				list.ThisUpdate, list.NextUpdate = now.Add(time.Hour), now.Add(2*time.Hour)
			case "matching scope":
				scope(crlScope(t, leaf.CRLDistributionPoints[0]))
			case "other scope":
				scope(crlScope(t, "http://example.test/other.crl"))
			case "partial reasons":
				scope([]byte{0x30, 4, 0x83, 2, 0, 0x40})
			case "indirect":
				scope([]byte{0x30, 3, 0x84, 1, 0xff})
			case "CA only":
				scope([]byte{0x30, 3, 0x82, 1, 0xff})
			case "delta":
				list.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 27}, Critical: true, Value: crlDER(t, 1)}}
			case "unknown critical":
				list.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{1, 2, 3, 4}, Critical: true, Value: []byte{5, 0}}}
			}
			data := testCRL(t, ca, key, list)
			if mode == "tampered" {
				data[len(data)-1] ^= 1
			}
			issuer := ca
			if mode == "other issuer" {
				other := *ca
				other.RawSubject = crlDER(t, pkix.Name{CommonName: "Other CA"}.ToRDNSequence())
				issuer = &other
			}
			err := validateCRL(data, leaf, issuer, now)
			wantOK := mode == "valid" || mode == "matching scope"
			if (err == nil) != wantOK {
				t.Fatalf("CRL error = %v, wantOK = %v", err, wantOK)
			}
		})
	}
}

func TestMSIVerifierEnforcesOnlineRevocation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"},
		NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	ca := testCertificate(t, key, caTemplate, caTemplate)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	for _, mode := range []string{"valid", "revoked publisher", "revoked timestamp", "unavailable CRL"} {
		t.Run(mode, func(t *testing.T) {
			var body []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if mode == "unavailable CRL" {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_, _ = w.Write(body)
			}))
			defer server.Close()
			leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2),
				Subject:   pkix.Name{CommonName: pokemonPublisher, Organization: []string{pokemonPublisher}},
				NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
				KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
				CRLDistributionPoints: []string{server.URL}}
			leaf := testCertificate(t, key, leafTemplate, ca)
			cert := &certloader.Certificate{Leaf: leaf, PrivateKey: key, Certificates: []*x509.Certificate{leaf, ca}}
			list := &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: now.Add(-time.Hour), NextUpdate: now.Add(time.Hour)}
			if mode == "revoked publisher" || mode == "revoked timestamp" {
				serial := int64(2)
				if mode == "revoked timestamp" {
					serial = 3
					tsaTemplate := *leafTemplate
					tsaTemplate.SerialNumber = big.NewInt(serial)
					tsaTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}
					tsaTemplate.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true,
						Value: crlDER(t, []asn1.ObjectIdentifier{pkcs9.OidKeyPurposeTimeStamping})}}
					tsa := testCertificate(t, key, &tsaTemplate, ca)
					cert.Timestamper = testTimestamper{&certloader.Certificate{Leaf: tsa, PrivateKey: key, Certificates: []*x509.Certificate{tsa, ca}}, now.Add(-10 * time.Minute)}
				}
				list.RevokedCertificateEntries = []x509.RevocationListEntry{{SerialNumber: big.NewInt(serial), RevocationTime: now.Add(-time.Minute)}}
			}
			body = testCRL(t, ca, key, list)
			path := unsignedMSI(t)
			signTestMSI(t, path, cert)
			err := verifyMSI(context.Background(), path, roots)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("verification error = %v for %s", err, mode)
			}
		})
	}
}
