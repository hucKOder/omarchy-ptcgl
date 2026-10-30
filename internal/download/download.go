// Package download safely transfers and verifies installation files.
package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Downloader struct {
	Client   *http.Client
	Progress func(ProgressEvent)
	Version  string
}

// ProgressEvent describes a download or verification phase.
type ProgressEvent struct {
	Phase          string  `json:"phase"`
	Message        string  `json:"message"`
	Downloaded     int64   `json:"downloaded"`
	Total          int64   `json:"total"` // -1 means unknown; never invent a percentage.
	BytesPerSecond float64 `json:"bytes_per_second"`
	Cancellable    bool    `json:"cancellable"`
}

type progressWriter struct {
	n, total      int64
	started, last time.Time
	notify        func(ProgressEvent)
}

func (w *progressWriter) report(force bool) {
	if w.notify == nil || (!force && time.Since(w.last) < 250*time.Millisecond) {
		return
	}
	w.last = time.Now()
	w.notify(ProgressEvent{
		Phase: "downloading", Message: "Downloading installation files…", Downloaded: w.n, Total: w.total,
		BytesPerSecond: float64(w.n) / time.Since(w.started).Seconds(), Cancellable: true,
	})
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	w.report(false)
	return len(p), nil
}

func NewDownloader(version string) Downloader {
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
	}, Version: version}
}

// Download publishes only after size, transfer length, and verifier checks pass.
// A previously valid cached installer remains available if a refresh fails.
func (d Downloader) Download(ctx context.Context, source, destination string, maxBytes int64, verify func(string) error) error {
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "https" {
		return fmt.Errorf("download requires HTTPS")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ptcgl-omarchy/"+d.Version)
	if d.Progress != nil {
		d.Progress(ProgressEvent{Phase: "downloading", Message: "Connecting to download server…", Total: -1, Cancellable: true})
	}
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
	progress := &progressWriter{total: resp.ContentLength, started: time.Now(), notify: d.Progress}
	progress.report(true)
	n, err := io.Copy(io.MultiWriter(f, progress), io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return fmt.Errorf("download interrupted: %w", err)
	}
	if n > maxBytes {
		return fmt.Errorf("download exceeds size limit")
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return fmt.Errorf("incomplete download")
	}
	progress.report(true)
	if err := ctx.Err(); err != nil {
		return err
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
	if d.Progress != nil {
		d.Progress(ProgressEvent{Phase: "verifying", Message: "Verifying download integrity and authenticity…", Total: -1})
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err = verify(f.Name()); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(f.Name(), destination)
}
