package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"ptcgl-omarchy/internal/download"
)

type Downloader = download.Downloader

func NewDownloader() Downloader { return download.NewDownloader(Version) }

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

func (a *App) download(ctx context.Context, source, destination string, maxBytes int64, verify func(string) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.mu.Lock()
	a.downloadCancel = cancel
	a.mu.Unlock()
	clear := func() { a.mu.Lock(); a.downloadCancel = nil; a.mu.Unlock() }
	defer clear()
	d := a.Download
	lastPhase := ""
	d.Progress = func(event ProgressEvent) {
		// User cancellation is scoped to the transfer, never an installer process.
		if !event.Cancellable {
			clear()
		}
		if event.Phase != lastPhase && event.Phase == "verifying" {
			a.progress(event.Message)
		}
		lastPhase = event.Phase
		a.emit(event)
	}
	if err := d.Download(ctx, source, destination, maxBytes, verify); err != nil {
		return fmt.Errorf("download or verification failed; retry with ptcgl-omarchy setup (or retry the selected maintenance action): %w", err)
	}
	return nil
}
