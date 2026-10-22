package app

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sassoftware/relic/v8/lib/authenticode"
	"github.com/sassoftware/relic/v8/lib/certloader"
	"github.com/sassoftware/relic/v8/lib/comdoc"
	"github.com/sassoftware/relic/v8/lib/pkcs7"
	"github.com/sassoftware/relic/v8/lib/pkcs9"
	"github.com/sassoftware/relic/v8/lib/x509tools"
)

// Make a minimal compound document, then let Relic add ordinary MSI streams.
// The fixture contains no game files and needs no external signing tools.
func unsignedMSI(t *testing.T) string {
	t.Helper()
	h := comdoc.Header{
		Magic:    [8]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1},
		Revision: 0x3e, Version: 3, ByteOrder: 0xfffe, SectorSize: 9, ShortSectorSize: 6,
		SATSectors: 1, DirNextSector: 1, MinStdStreamSize: 4096,
		SSATNextSector: 11, SSATSectorCount: 1, MSATNextSector: comdoc.SecIDEndOfChain,
	}
	for i := range h.MSAT {
		h.MSAT[i] = comdoc.SecIDFree
	}
	h.MSAT[0] = 0
	var fat [128]comdoc.SecID
	for i := range fat {
		fat[i] = comdoc.SecIDFree
	}
	fat[0], fat[1] = comdoc.SecIDSAT, comdoc.SecIDEndOfChain
	for i := 2; i < 9; i++ {
		fat[i] = comdoc.SecID(i + 1)
	}
	fat[9] = comdoc.SecIDEndOfChain
	fat[10], fat[11] = comdoc.SecIDEndOfChain, comdoc.SecIDEndOfChain
	root := comdoc.RawDirEnt{Type: comdoc.DirRoot, Color: comdoc.Black,
		NameLength: 22, LeftChild: -1, RightChild: -1, StorageRoot: 1, NextSector: 10, StreamSize: 512}
	for i, r := range "Root Entry" {
		root.NameRunes[i] = uint16(r)
	}
	payload := comdoc.RawDirEnt{Type: comdoc.DirStream, Color: comdoc.Black,
		NameLength: 16, LeftChild: -1, RightChild: -1, StorageRoot: -1, NextSector: 2, StreamSize: 4096}
	for i, r := range "Payload" {
		payload.NameRunes[i] = uint16(r)
	}
	var b bytes.Buffer
	for _, value := range []any{h, fat, root, payload, [256]byte{}} {
		if err := binary.Write(&b, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	b.Write(bytes.Repeat([]byte("original"), 512))
	b.Write(make([]byte, 512))
	var shortFAT [128]comdoc.SecID
	for i := range shortFAT {
		shortFAT[i] = comdoc.SecIDFree
	}
	if err := binary.Write(&b, binary.LittleEndian, shortFAT); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test installer.msi")
	writeTestFile(t, path, b.Bytes())
	return path
}

func modifyMSI(t *testing.T, path, name string, content []byte) {
	t.Helper()
	cdf, err := comdoc.WritePath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cdf.AddFile(name, content); err != nil {
		t.Fatal(err)
	}
	if err := cdf.Close(); err != nil {
		t.Fatal(err)
	}
}

func testCertificate(t *testing.T, key *rsa.PrivateKey, template, parent *x509.Certificate) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

type testTimestamper struct {
	cert *certloader.Certificate
	time time.Time
}

func (s testTimestamper) Timestamp(_ context.Context, req *pkcs9.Request) (*pkcs7.ContentInfoSignedData, error) {
	hash := req.Hash.New()
	hash.Write(req.EncryptedDigest)
	algorithm, _ := x509tools.PkixDigestAlgorithm(req.Hash)
	encodedTime, err := asn1.MarshalWithParams(s.time.UTC(), "generalized")
	if err != nil {
		return nil, err
	}
	info := pkcs9.TSTInfo{Version: 1, Policy: asn1.ObjectIdentifier{1, 2, 3, 4},
		MessageImprint: pkcs9.MessageImprint{HashAlgorithm: algorithm, HashedMessage: hash.Sum(nil)},
		SerialNumber:   big.NewInt(1), GenTime: asn1.RawValue{FullBytes: encodedTime}}
	encodedInfo, err := asn1.Marshal(info)
	if err != nil {
		return nil, err
	}
	builder := pkcs7.NewBuilder(s.cert.Signer(), s.cert.Chain(), req.Hash)
	if err := builder.SetContent(pkcs9.OidTSTInfo, encodedInfo); err != nil {
		return nil, err
	}
	return builder.Sign()
}

func signTestMSI(t *testing.T, path string, cert *certloader.Certificate) {
	t.Helper()
	cdf, err := comdoc.WritePath(path)
	if err != nil {
		t.Fatal(err)
	}
	digest, _, err := authenticode.DigestMSI(cdf, crypto.SHA256, false)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := authenticode.SignMSIImprint(context.Background(), digest, crypto.SHA256, cert, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := authenticode.InsertMSISignature(cdf, sig.Raw, nil); err != nil {
		t.Fatal(err)
	}
	if err := cdf.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltInMSIVerification(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: pokemonPublisher},
		NotBefore: now.Add(-365 * 24 * time.Hour), NotAfter: now.Add(365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	ca := testCertificate(t, key, caTemplate, caTemplate)
	crl := testCRL(t, ca, key, &x509.RevocationList{
		Number: big.NewInt(1), ThisUpdate: now.Add(-time.Hour), NextUpdate: now.Add(time.Hour),
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(crl) }))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	for _, mode := range []string{"valid", "tampered", "unsigned", "untrusted", "wrong publisher", "lookalike publisher", "wrong organization", "wrong usage", "wrong key usage", "expired", "timestamped expired", "untrusted timestamp", "invalid timestamp usage", "future timestamp", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			path := unsignedMSI(t)
			leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2),
				Subject:   pkix.Name{CommonName: pokemonPublisher, Organization: []string{pokemonPublisher}},
				NotBefore: now.Add(-3 * 24 * time.Hour), NotAfter: now.Add(24 * time.Hour),
				KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
				CRLDistributionPoints: []string{server.URL}}
			switch mode {
			case "wrong publisher":
				leafTemplate.Subject.CommonName = "Other Publisher"
			case "lookalike publisher":
				leafTemplate.Subject.CommonName += " Evil"
			case "wrong organization":
				leafTemplate.Subject.Organization = []string{"Other Publisher"}
			case "wrong usage":
				leafTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			case "wrong key usage":
				leafTemplate.KeyUsage = x509.KeyUsageKeyEncipherment
			case "expired", "timestamped expired", "untrusted timestamp":
				leafTemplate.NotAfter = now.Add(-24 * time.Hour)
			}
			leaf := testCertificate(t, key, leafTemplate, ca)
			cert := &certloader.Certificate{Leaf: leaf, PrivateKey: key, Certificates: []*x509.Certificate{leaf, ca}}
			if mode == "timestamped expired" || mode == "untrusted timestamp" || mode == "future timestamp" || mode == "invalid timestamp usage" {
				tsaTemplate := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "Timestamp authority"},
					NotBefore: now.Add(-365 * 24 * time.Hour), NotAfter: now.Add(365 * 24 * time.Hour),
					KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
					CRLDistributionPoints: []string{server.URL}}
				if mode != "invalid timestamp usage" {
					tsaTemplate.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true,
						Value: crlDER(t, []asn1.ObjectIdentifier{pkcs9.OidKeyPurposeTimeStamping})}}
				}
				parent := ca
				if mode == "untrusted timestamp" {
					other := *caTemplate
					other.SerialNumber, other.Subject = big.NewInt(4), pkix.Name{CommonName: "Untrusted CA"}
					parent = testCertificate(t, key, &other, &other)
				}
				tsa := testCertificate(t, key, tsaTemplate, parent)
				stampTime := now.Add(-48 * time.Hour)
				if mode == "future timestamp" {
					stampTime = now.Add(time.Hour)
				}
				cert.Timestamper = testTimestamper{&certloader.Certificate{Leaf: tsa, PrivateKey: key, Certificates: []*x509.Certificate{tsa, parent}}, stampTime}
			}
			if mode != "unsigned" {
				signTestMSI(t, path, cert)
			}
			if mode == "tampered" {
				modifyMSI(t, path, "Payload", []byte("replaced after signing"))
			}
			trusted := roots
			if mode == "untrusted" {
				trusted = x509.NewCertPool()
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			err := verifyMSI(ctx, path, trusted)
			wantOK := mode == "valid" || mode == "timestamped expired"
			if (err == nil) != wantOK {
				t.Fatalf("verification error = %v, wantOK = %v", err, wantOK)
			}
		})
	}
}

func TestVerificationWorkerNeedsNoLauncherConfiguration(t *testing.T) {
	for _, args := range [][]string{{"__verify-installer"}, {"__verify-installer", "missing.msi", "extra"}, {"__verify-installer", "missing.msi"}} {
		var out bytes.Buffer
		if code := RunCLI(context.Background(), args, io.Discard, &out); code == 0 {
			t.Fatalf("worker accepted invalid invocation: %v", args)
		}
	}
	path := filepath.Join(t.TempDir(), "malformed.msi")
	writeTestFile(t, path, []byte("not an MSI"))
	if err := verifyMSI(context.Background(), path, nil); err == nil {
		t.Fatal("accepted malformed MSI")
	}
}

func TestMSIVerificationUsesBundledWorker(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "worker-test-placeholder")
	a, r := testApp(t)
	path := filepath.Join(t.TempDir(), "installer.msi")
	writeTestFile(t, path, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	a.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if err := a.verifyInstaller(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	if len(r.commands) != 1 || r.commands[0].Name != self || strings.Join(r.commands[0].Args, "|") != "__verify-installer|"+path {
		t.Fatalf("not using the bundled verifier: %+v", r.commands)
	}
	if r.commands[0].Env == nil || contains(r.commands[0].Env, "GITHUB_TOKEN=worker-test-placeholder") {
		t.Fatal("MSI worker inherited developer credentials")
	}
}
