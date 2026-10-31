package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"lazygram/internal/update"
)

// releaseServer answers like GitHub: releases/latest redirects to the tag
// of the latest release. Every download request is counted in downloads
// (when not nil) and answered 404, so a test can tell that nothing was
// fetched.
func releaseServer(t *testing.T, latest string, downloads *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/"+update.Repo+"/releases/latest":
			http.Redirect(w, r, "/"+update.Repo+"/releases/tag/"+latest, http.StatusFound)
		case strings.Contains(r.URL.Path, "/releases/download/"):
			if downloads != nil {
				downloads.Add(1)
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestUpdateRefusesAPackagedInstall(t *testing.T) {
	var downloads atomic.Int32
	srv := releaseServer(t, "v9.9.9", &downloads)
	defer srv.Close()
	c := update.New()
	c.Base = srv.URL
	packaged := func() (string, error) { return "/usr/bin/lazygram", nil }

	var out bytes.Buffer
	err := runUpdate(c, "0.4.14", packaged, false, true, strings.NewReader(""), &out)
	if !errors.Is(err, update.ErrPackaged) || !strings.Contains(err.Error(), "/usr/bin/lazygram") {
		t.Fatalf("packaged install: %v", err)
	}
	if strings.Contains(out.String(), "[y/N]") || !strings.Contains(out.String(), "update available: v0.4.14 -> v9.9.9") {
		t.Fatalf("output: %q", out.String())
	}
	if downloads.Load() != 0 {
		t.Fatalf("%d download(s) for a packaged install", downloads.Load())
	}

	// -check only reports, for a packaged install too.
	out.Reset()
	if err := runUpdate(c, "0.4.14", packaged, true, false, strings.NewReader(""), &out); err != nil || !strings.Contains(out.String(), "update available") {
		t.Fatalf("check: %v %q", err, out.String())
	}

	// A plain binary is asked; "n" skips, and nothing is downloaded.
	out.Reset()
	plain := func() (string, error) { return "/usr/local/bin/lazygram", nil }
	if err := runUpdate(c, "0.4.14", plain, false, false, strings.NewReader("n\n"), &out); err != nil || !strings.Contains(out.String(), "(binary)? [y/N] skipped") {
		t.Fatalf("declined: %v %q", err, out.String())
	}
	if downloads.Load() != 0 {
		t.Fatalf("%d download(s) after declining", downloads.Load())
	}

	// Up to date and builds from source are reported before any path check.
	out.Reset()
	if err := runUpdate(c, "v9.9.9", packaged, false, true, strings.NewReader(""), &out); err != nil || !strings.Contains(out.String(), "up to date") {
		t.Fatalf("up to date: %v %q", err, out.String())
	}
	if err := runUpdate(c, "dev", packaged, false, true, strings.NewReader(""), &out); err == nil || !strings.Contains(err.Error(), "build from source") {
		t.Fatalf("source build: %v", err)
	}
}
