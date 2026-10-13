package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Downloader struct{ Client *http.Client }

func NewDownloader() Downloader {
	return Downloader{Client: &http.Client{
		Timeout: 30 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("refusing non-HTTPS redirect")
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Download publishes only after size, transfer length, and verifier checks pass.
// A previously valid cached installer remains available if a refresh fails.
func (d Downloader) Download(ctx context.Context, source, destination string, maxBytes int64, verify func(string) error) error {
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "https" {
		return fmt.Errorf("download requires HTTPS")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ptcgl-omarchy/"+Version)
	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return fmt.Errorf("download exceeds size limit")
	}
	f, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return fmt.Errorf("download interrupted: %w", err)
	}
	if n > maxBytes {
		return fmt.Errorf("download exceeds size limit")
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return fmt.Errorf("incomplete download")
	}
	if n == 0 {
		return fmt.Errorf("empty download")
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = verify(f.Name()); err != nil {
		return err
	}
	return os.Rename(f.Name(), destination)
}
