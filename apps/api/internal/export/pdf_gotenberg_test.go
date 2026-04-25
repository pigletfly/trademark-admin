package export_test

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func TestGotenberg_RenderPDF_PostsMultipartReturnsBody(t *testing.T) {
	var receivedHTML []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/chromium/convert/html" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "multipart/form-data") {
			t.Errorf("not multipart: %s", ct)
		}
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("next part: %v", err)
			}
			if p.FormName() == "files" {
				receivedHTML, _ = io.ReadAll(p)
			}
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-fake"))
	}))
	defer srv.Close()

	g := export.NewGotenberg(srv.URL)
	pdf, err := g.RenderPDF(context.Background(), []byte("<html>hi</html>"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("not PDF: %q", pdf)
	}
	if !bytes.Contains(receivedHTML, []byte("<html>hi</html>")) {
		t.Fatalf("html not forwarded: %q", receivedHTML)
	}
}

func TestGotenberg_Error_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	g := export.NewGotenberg(srv.URL)
	_, err := g.RenderPDF(context.Background(), []byte("x"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGotenberg_ContextCancelled(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done(): // caller cancelled
		case <-done: // test is tearing down
		}
	}))
	defer func() {
		close(done)
		srv.Close()
	}()

	g := export.NewGotenberg(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := g.RenderPDF(ctx, []byte("x"))
	if err == nil {
		t.Fatal("expected timeout/cancel error")
	}
}
