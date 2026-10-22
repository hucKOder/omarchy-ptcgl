package app

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sassoftware/relic/v8/lib/authenticode"
	"github.com/sassoftware/relic/v8/lib/pkcs7"
)

const pokemonPublisher = "The Pokemon Company International, Inc."

type verificationReader struct {
	ctx context.Context
	io.ReaderAt
}

func (r verificationReader) ReadAt(p []byte, off int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.ReaderAt.ReadAt(p, off)
}

// roots is nil in production, selecting the system certificate authorities.
// Tests use an isolated CA rather than trusting self-signed test publishers.
func verifyMSI(ctx context.Context, path string, roots *x509.CertPool) (err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("malformed MSI signature data")
		}
	}()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<30 {
		return fmt.Errorf("installer must be a regular MSI file no larger than 1 GiB")
	}
	sig, err := authenticode.VerifyMSI(verificationReader{ctx, f}, false)
	if err != nil {
		return fmt.Errorf("installer signature verification failed: %w", err)
	}
	if sig.CounterSignature != nil && sig.CounterSignature.SigningTime.After(time.Now().Add(5*time.Minute)) {
		return fmt.Errorf("installer signature timestamp is in the future")
	}
	subject := sig.Signature.Certificate.Subject
	if subject.CommonName != pokemonPublisher || len(subject.Organization) != 1 || subject.Organization[0] != pokemonPublisher {
		return fmt.Errorf("installer is not signed by Pokémon's publisher")
	}
	// VerifyMSI checks the signature and MSI digest, but not certificate trust.
	// Trust the timestamp authority before using its time for the publisher.
	var signedAt time.Time
	if stamp := sig.CounterSignature; stamp != nil {
		if err := verifyCertificate(ctx, stamp.Signature, roots, x509.ExtKeyUsageTimeStamping, stamp.SigningTime); err != nil {
			return fmt.Errorf("installer timestamp certificate verification failed: %w", err)
		}
		signedAt = stamp.SigningTime
	}
	if err := verifyCertificate(ctx, sig.Signature, roots, x509.ExtKeyUsageCodeSigning, signedAt); err != nil {
		return fmt.Errorf("installer certificate verification failed: %w", err)
	}
	return ctx.Err()
}

func verifyCertificate(ctx context.Context, sig pkcs7.Signature, roots *x509.CertPool, usage x509.ExtKeyUsage, signedAt time.Time) error {
	cert := sig.Certificate
	if cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return fmt.Errorf("certificate key usage does not permit digital signatures")
	}
	if usage == x509.ExtKeyUsageTimeStamping {
		critical := false
		for _, extension := range cert.Extensions {
			critical = critical || (extension.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 37}) && extension.Critical)
		}
		if !critical || len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageTimeStamping || len(cert.UnknownExtKeyUsage) != 0 {
			return fmt.Errorf("timestamp certificate must have critical, exclusive timestamping usage")
		}
	}
	intermediates := x509.NewCertPool()
	for _, cert := range sig.Intermediates {
		intermediates.AddCert(cert)
	}
	chains, err := cert.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates,
		KeyUsages: []x509.ExtKeyUsage{usage}, CurrentTime: signedAt})
	if err != nil {
		return err
	}
	return verifyChainRevocation(ctx, chains)
}

// Certificate.Verify does not check revocation. Require current evidence for
// every non-root certificate, even when validity was evaluated at signing time.
// A cross-signed intermediate may have several paths: one complete, non-revoked
// path is sufficient. Trust anchors are maintained by the system trust store.
func verifyChainRevocation(ctx context.Context, chains [][]*x509.Certificate) error {
	type pair struct{ certificate, issuer string }
	checked := make(map[pair]error)
	var failures []error
	for _, chain := range chains {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(chain) == 0 {
			continue
		}
		var chainErr error
		for i := 0; i+1 < len(chain); i++ {
			cert, issuer := chain[i], chain[i+1]
			key := pair{string(cert.Raw), string(issuer.Raw)}
			err, found := checked[key]
			if !found {
				err = verifyRevocation(ctx, cert, issuer)
				checked[key] = err
			}
			if err != nil {
				chainErr = fmt.Errorf("certificate %q: %w", cert.Subject.CommonName, err)
				break
			}
		}
		if chainErr == nil {
			return ctx.Err()
		}
		failures = append(failures, chainErr)
	}
	if len(failures) == 0 {
		return fmt.Errorf("no certificate chain to check")
	}
	return fmt.Errorf("no certificate chain has complete revocation evidence: %w", errors.Join(failures...))
}
