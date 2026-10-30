package verify

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sassoftware/relic/v8/lib/certloader"
	"github.com/sassoftware/relic/v8/lib/pkcs7"
	"github.com/sassoftware/relic/v8/lib/pkcs9"
)

func issueCertificate(t *testing.T, template, parent *x509.Certificate, key, parentKey *rsa.PrivateKey) *x509.Certificate {
	t.Helper()
	if parent == nil {
		parent, parentKey = template, key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func securityKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestMSIRequiresRevocationAcrossPublisherAndTimestampChains(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	rootKey, intermediateKey, leafKey := securityKey(t), securityKey(t), securityKey(t)
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Security test root"},
		NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	root := issueCertificate(t, rootTemplate, nil, rootKey, nil)
	roots := x509.NewCertPool()
	roots.AddCert(root)
	for _, mode := range []string{
		"valid", "revoked publisher intermediate", "revoked timestamp intermediate",
		"unavailable intermediate CRL", "stale intermediate CRL", "missing intermediate CDP",
		"missing publisher CDP", "missing timestamp CDP",
	} {
		t.Run(mode, func(t *testing.T) {
			bodies := make(map[string][]byte)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, ok := bodies[r.URL.Path]
				if !ok {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_, _ = w.Write(body)
			}))
			defer server.Close()
			intermediate := func(serial int64, name string) *x509.Certificate {
				points := []string{server.URL + "/root.crl"}
				if mode == "missing intermediate CDP" {
					points = nil
				}
				return issueCertificate(t, &x509.Certificate{
					SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
					NotBefore: now.Add(-12 * time.Hour), NotAfter: now.Add(12 * time.Hour),
					IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
					CRLDistributionPoints: points,
				}, root, intermediateKey, rootKey)
			}
			publisherCA := intermediate(2, "Publisher intermediate")
			timestampCA := intermediate(3, "Timestamp intermediate")
			publisherPoints := []string{server.URL + "/publisher.crl"}
			if mode == "missing publisher CDP" {
				publisherPoints = nil
			}
			publisher := issueCertificate(t, &x509.Certificate{
				SerialNumber: big.NewInt(4),
				Subject:      pkix.Name{CommonName: pokemonPublisher, Organization: []string{pokemonPublisher}},
				NotBefore:    now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
				KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
				CRLDistributionPoints: publisherPoints,
			}, publisherCA, leafKey, intermediateKey)
			timestampPoints := []string{server.URL + "/timestamp.crl"}
			if mode == "missing timestamp CDP" {
				timestampPoints = nil
			}
			timestamp := issueCertificate(t, &x509.Certificate{
				SerialNumber: big.NewInt(5), Subject: pkix.Name{CommonName: "Timestamp signer"},
				NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
				KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
				ExtraExtensions: []pkix.Extension{{
					Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true,
					Value: crlDER(t, []asn1.ObjectIdentifier{pkcs9.OidKeyPurposeTimeStamping}),
				}},
				CRLDistributionPoints: timestampPoints,
			}, timestampCA, leafKey, intermediateKey)
			list := &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: now.Add(-time.Hour), NextUpdate: now.Add(time.Hour)}
			bodies["/publisher.crl"] = testCRL(t, publisherCA, intermediateKey, list)
			bodies["/timestamp.crl"] = testCRL(t, timestampCA, intermediateKey, list)
			if strings.HasPrefix(mode, "revoked ") {
				serial := publisherCA.SerialNumber
				if mode == "revoked timestamp intermediate" {
					serial = timestampCA.SerialNumber
				}
				list.RevokedCertificateEntries = []x509.RevocationListEntry{{SerialNumber: serial, RevocationTime: now.Add(-time.Hour)}}
			}
			if mode == "stale intermediate CRL" {
				list.NextUpdate = now.Add(-time.Minute)
			}
			if mode != "unavailable intermediate CRL" {
				bodies["/root.crl"] = testCRL(t, root, rootKey, list)
			}
			cert := &certloader.Certificate{
				Leaf: publisher, PrivateKey: leafKey,
				Certificates: []*x509.Certificate{publisher, publisherCA, root},
				Timestamper: testTimestamper{&certloader.Certificate{
					Leaf: timestamp, PrivateKey: leafKey,
					Certificates: []*x509.Certificate{timestamp, timestampCA, root},
				}, now.Add(-10 * time.Minute)},
			}
			path := unsignedMSI(t)
			signTestMSI(t, path, cert)
			err := verifyMSI(context.Background(), path, roots)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("MSI verification error = %v", err)
			}
			if strings.HasPrefix(mode, "revoked ") && !errors.Is(err, errRevoked) {
				t.Fatalf("expected revocation rejection, got %v", err)
			}
		})
	}
}

func TestRevocationUsesAnEntireValidAlternateChain(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	key := securityKey(t)
	bodies := make(map[string][]byte)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(bodies[r.URL.Path]) }))
	defer server.Close()
	root := func(serial int64, name string) *x509.Certificate {
		return issueCertificate(t, &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
			KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		}, nil, key, nil)
	}
	first, second := root(1, "First root"), root(2, "Second root")
	intermediate := func(serial int64, parent *x509.Certificate, path string) *x509.Certificate {
		return issueCertificate(t, &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "Cross-signed intermediate"},
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
			KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, CRLDistributionPoints: []string{server.URL + path},
		}, parent, key, key)
	}
	bad, good := intermediate(3, first, "/first.crl"), intermediate(4, second, "/second.crl")
	leaf := issueCertificate(t, &x509.Certificate{
		SerialNumber: big.NewInt(5), Subject: pkix.Name{CommonName: pokemonPublisher},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		CRLDistributionPoints: []string{server.URL + "/leaf.crl"},
	}, bad, key, key)
	clear := &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: now.Add(-time.Minute), NextUpdate: now.Add(time.Hour)}
	bodies["/leaf.crl"] = testCRL(t, bad, key, clear)
	bodies["/second.crl"] = testCRL(t, second, key, clear)
	revoked := *clear
	revoked.RevokedCertificateEntries = []x509.RevocationListEntry{{SerialNumber: bad.SerialNumber, RevocationTime: now.Add(-time.Minute)}}
	bodies["/first.crl"] = testCRL(t, first, key, &revoked)
	chains := [][]*x509.Certificate{{leaf, bad, first}, {leaf, good, second}}
	if err := verifyChainRevocation(context.Background(), chains[:1]); !errors.Is(err, errRevoked) {
		t.Fatalf("bad chain accepted: %v", err)
	}
	if err := verifyChainRevocation(context.Background(), chains); err != nil {
		t.Fatalf("valid alternative rejected: %v", err)
	}
	roots, intermediates := x509.NewCertPool(), x509.NewCertPool()
	roots.AddCert(first)
	roots.AddCert(second)
	intermediates.AddCert(bad)
	intermediates.AddCert(good)
	verified, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}})
	if err != nil || len(verified) != 2 {
		t.Fatalf("fixture must have two valid X.509 paths: %v", err)
	}
	if err := verifyCertificate(context.Background(), pkcs7.Signature{Certificate: leaf, Intermediates: []*x509.Certificate{bad, good}}, roots, x509.ExtKeyUsageCodeSigning, time.Time{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := verifyChainRevocation(ctx, chains); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}
