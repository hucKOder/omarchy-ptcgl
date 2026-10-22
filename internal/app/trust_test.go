package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func compromisedTLSFixture(t *testing.T, host string) ([]*x509.Certificate, *x509.CertPool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "emudhra", host+".pem"))
	if err != nil {
		t.Fatal(err)
	}
	var chain []*x509.Certificate
	for len(bytes.TrimSpace(data)) > 0 {
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			t.Fatal("invalid public certificate-chain fixture")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		chain = append(chain, cert)
		data = rest
	}
	if len(chain) < 2 {
		t.Fatal("public fixture must include a leaf and intermediate")
	}
	data, err = os.ReadFile(filepath.Join("testdata", "emudhra", "roots.pem"))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		t.Fatal("invalid public root fixture")
	}
	return chain, roots
}

func requireCompromisedKeyRejection(t *testing.T, err error) {
	t.Helper()
	var invalid x509.CertificateInvalidError
	// Go 1.27.3 filters these otherwise-valid chains at its SPKI denylist.
	// Expiry, hostname, unknown-root, policy and usage failures must not pass.
	if !errors.As(err, &invalid) || invalid.Reason != x509.NoValidChains || invalid.Detail != "" {
		t.Fatalf("expected compromised-key chain filtering, got %v", err)
	}
}

func TestCompromisedPublicTLSChainsAreDistrusted(t *testing.T) {
	for _, host := range []string{"dv-g3b-emudhra.we-have-a-problem.com", "dv-g3b-aaa-emudhra.we-have-a-problem.com"} {
		t.Run(host, func(t *testing.T) {
			chain, roots := compromisedTLSFixture(t, host)
			intermediates := x509.NewCertPool()
			for _, cert := range chain[1:] {
				intermediates.AddCert(cert)
			}
			_, err := chain[0].Verify(x509.VerifyOptions{
				Roots: roots, Intermediates: intermediates, DNSName: host,
				CurrentTime: time.Date(2026, time.October, 22, 12, 0, 0, 0, time.UTC),
			})
			requireCompromisedKeyRejection(t, err)
		})
	}
}

func TestDownloadRejectsUntrustedTLSBeforeReplacingCache(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("untrusted")) }))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "cached")
	writeTestFile(t, destination, []byte("previously verified"))
	verified := false
	err := NewDownloader().Download(context.Background(), server.URL, destination, 1024, func(string) error { verified = true; return nil })
	var certificateError *tls.CertificateVerificationError
	if !errors.As(err, &certificateError) || verified {
		t.Fatalf("untrusted TLS was not rejected before content verification: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "previously verified" {
		t.Fatalf("TLS failure damaged the cache: %v", err)
	}
}

// Optional real-verifier smoke test. These hosts intentionally serve chains
// using the publicly compromised eMudhra keys; no private key is downloaded.
// Keep ordinary unit tests independent of an external incident-response service.
func TestPublicTLSRejectsCompromisedIntermediates(t *testing.T) {
	if os.Getenv("PTCGL_TEST_PUBLIC_TRUST") != "1" {
		t.Skip("set PTCGL_TEST_PUBLIC_TRUST=1 for the public eMudhra TLS smoke test")
	}
	keys := map[string]bool{
		"ebed6475c2c2d80fcb199f3fe8dd6e6b3ce60b77eb15d4e42c3dec0c6f5b5105": true,
		"49dd353dc2b919a3770645117d0e60c16115e7d1bc259eefe734b00ea434fe5f": true,
		"e0254c7fcdc265e2f0f8a428dabb75ab4ae039b4872d62f5ee57f0877d5a8a41": true,
		"5c0660a2b164910d8ff589c18e14552e9e0ec237dcefe2cb736d41a4811f81a0": true,
		"43bffa19d6ef651451f6841e019931e96f6f2eaeff28ab4b6f1ce4576de2717b": true,
		"63ecefee431d49523bc1136c4e72d0206a477086caf606cd5cf025686c109cf8": true,
		"ec4ffa1ad0ac88081314b7aaffef6a91f97cd57291bb5ba686c5415f5eb4f6b6": true,
		"92c063d67e8f14db71d288e11266ffdaf69cbf115f7bab14f3bc2f68d25e0e30": true,
	}
	for _, host := range []string{"dv-g3b-emudhra.we-have-a-problem.com", "dv-g3b-aaa-emudhra.we-have-a-problem.com"} {
		t.Run(host, func(t *testing.T) {
			// Explicit test-only roots prevent system root removal from masking
			// a missing runtime key denylist. Production still uses system roots.
			_, roots := compromisedTLSFixture(t, host)
			downloader := NewDownloader()
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{RootCAs: roots}
			defer transport.CloseIdleConnections()
			downloader.Client.Transport = transport
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			destination := filepath.Join(t.TempDir(), "rejected")
			err := downloader.Download(ctx, "https://"+host+"/", destination, 16<<10, func(string) error { return nil })
			var certificateError *tls.CertificateVerificationError
			if !errors.As(err, &certificateError) {
				t.Fatalf("need a certificate rejection, not success or a network failure: %v", err)
			}
			requireCompromisedKeyRejection(t, certificateError.Err)
			matched := false
			for _, cert := range certificateError.UnverifiedCertificates {
				matched = matched || keys[fmt.Sprintf("%x", sha256.Sum256(cert.RawSubjectPublicKeyInfo))]
			}
			if !matched {
				t.Fatal("the rejected chain did not contain an expected compromised public key")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("a compromised TLS chain produced a published download")
			}
		})
	}
}
