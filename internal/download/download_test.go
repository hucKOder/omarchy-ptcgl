package download

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestVerificationBeforePublication(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "ptcgl-omarchy/test" {
			t.Errorf("unexpected user agent %q", got)
		}
		_, _ = w.Write([]byte("new"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "installer")
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := NewDownloader("test")
	d.Client.Transport = server.Client().Transport
	verificationErr := errors.New("verification failed")
	err := d.Download(context.Background(), server.URL, destination, 100, func(path string) error {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "new" {
			t.Fatalf("verification input %q: %v", got, err)
		}
		return verificationErr
	})
	if !errors.Is(err, verificationErr) {
		t.Fatalf("got %v, want verification failure", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "old" {
		t.Fatalf("cached file %q: %v", got, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".download-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary downloads %v: %v", leftovers, err)
	}
}

func TestRejectsPlaintextAndCancelledDownloads(t *testing.T) {
	d := NewDownloader("test")
	destination := filepath.Join(t.TempDir(), "installer")
	if err := d.Download(context.Background(), "http://example.com", destination, 100, func(string) error { return nil }); err == nil {
		t.Fatal("accepted plaintext download")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := d.Download(ctx, "https://example.com", destination, 100, func(string) error {
		t.Error("verification called after cancellation")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context cancellation", err)
	}
}
