package verify

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var errRevoked = errors.New("signing certificate has been revoked")

func verifyRevocation(ctx context.Context, cert, issuer *x509.Certificate) error {
	if len(cert.CRLDistributionPoints) == 0 {
		return fmt.Errorf("certificate has no CRL distribution point")
	}
	var lastErr error
	for _, point := range cert.CRLDistributionPoints {
		if err := ctx.Err(); err != nil {
			return err
		}
		// CRLs are signed data. HTTP CDPs remain usable, but try HTTPS first
		// where available. Neither transport can bypass issuer/signature checks.
		locations := []string{point}
		if strings.HasPrefix(point, "http://") {
			locations = []string{"https://" + strings.TrimPrefix(point, "http://"), point}
		}
		for _, location := range locations {
			data, err := downloadCRL(ctx, location)
			if err == nil {
				err = validateCRL(data, cert, issuer, time.Now())
			}
			if err == nil || errors.Is(err, errRevoked) {
				return err
			}
			lastErr = err
		}
	}
	return fmt.Errorf("could not verify certificate revocation: %w", lastErr)
}

func downloadCRL(ctx context.Context, location string) ([]byte, error) {
	u, err := url.Parse(location)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("unsupported CRL distribution point")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CRL download returned HTTP %d", resp.StatusCode)
	}
	const limit = 32 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("CRL exceeds 32 MiB")
	}
	return data, nil
}

func validateCRL(data []byte, cert, issuer *x509.Certificate, now time.Time) error {
	if block, _ := pem.Decode(data); block != nil {
		if block.Type != "X509 CRL" {
			return fmt.Errorf("unexpected PEM type for CRL")
		}
		data = block.Bytes
	}
	crl, err := x509.ParseRevocationList(data)
	if err != nil {
		return err
	}
	if !bytes.Equal(crl.RawIssuer, issuer.RawSubject) {
		return fmt.Errorf("CRL issuer does not match certificate issuer")
	}
	if err := crl.CheckSignatureFrom(issuer); err != nil {
		return fmt.Errorf("invalid CRL signature: %w", err)
	}
	// Establish coverage before interpreting an absent serial as non-revoked.
	// Delta, indirect, and reason-restricted lists are not complete evidence.
	for _, extension := range crl.Extensions {
		if extension.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 28}) {
			if err := validateCRLScope(extension.Value, cert); err != nil {
				return err
			}
		} else if extension.Critical || extension.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 27}) {
			return fmt.Errorf("unsupported CRL extension %s", extension.Id)
		}
	}
	if crl.ThisUpdate.After(now.Add(5*time.Minute)) || !crl.NextUpdate.After(crl.ThisUpdate) || !now.Before(crl.NextUpdate) {
		return fmt.Errorf("CRL is not current")
	}
	for _, entry := range crl.RevokedCertificateEntries {
		for _, extension := range entry.Extensions {
			if extension.Critical {
				return fmt.Errorf("unsupported critical CRL entry extension %s", extension.Id)
			}
		}
		if entry.SerialNumber.Cmp(cert.SerialNumber) == 0 {
			return errRevoked
		}
	}
	return nil
}

// RFC 5280 issuingDistributionPoint. Accept a full list or a named list whose
// URI matches this certificate's CDP, with optional user/CA type restrictions.
// Reject unsupported scope constraints instead of assuming full coverage.
func validateCRLScope(data []byte, cert *x509.Certificate) error {
	var sequence asn1.RawValue
	rest, err := asn1.Unmarshal(data, &sequence)
	if err != nil || len(rest) != 0 || sequence.Class != 0 || sequence.Tag != asn1.TagSequence || !sequence.IsCompound {
		return fmt.Errorf("invalid issuing distribution point")
	}
	seen := make(map[int]bool)
	for data = sequence.Bytes; len(data) > 0; {
		var field asn1.RawValue
		data, err = asn1.Unmarshal(data, &field)
		if err != nil || field.Class != 2 || seen[field.Tag] {
			return fmt.Errorf("invalid issuing distribution point field")
		}
		seen[field.Tag] = true
		if field.Tag == 0 {
			var name asn1.RawValue
			rest, err := asn1.Unmarshal(field.Bytes, &name)
			if err != nil || len(rest) != 0 || !field.IsCompound || name.Class != 2 || name.Tag != 0 || !name.IsCompound {
				return fmt.Errorf("unsupported CRL distribution point name")
			}
			matched := false
			for names := name.Bytes; len(names) > 0; {
				var uri asn1.RawValue
				names, err = asn1.Unmarshal(names, &uri)
				if err != nil || uri.Class != 2 || uri.Tag != 6 || uri.IsCompound {
					return fmt.Errorf("unsupported CRL distribution point URI")
				}
				for _, point := range cert.CRLDistributionPoints {
					matched = matched || string(uri.Bytes) == point
				}
			}
			if !matched {
				return fmt.Errorf("CRL distribution point does not cover this certificate")
			}
			continue
		}
		if field.Tag != 1 && field.Tag != 2 && field.Tag != 4 && field.Tag != 5 {
			return fmt.Errorf("unsupported CRL scope restriction")
		}
		if field.IsCompound || len(field.Bytes) != 1 || (field.Bytes[0] != 0 && field.Bytes[0] != 0xff) {
			return fmt.Errorf("invalid CRL scope flag")
		}
		if field.Bytes[0] == 0 {
			continue
		}
		if (field.Tag == 1 && cert.IsCA) || (field.Tag == 2 && !cert.IsCA) || field.Tag == 4 || field.Tag == 5 {
			return fmt.Errorf("CRL scope does not cover this certificate")
		}
	}
	return nil
}
