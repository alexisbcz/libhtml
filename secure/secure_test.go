package secure_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	h "github.com/alexisbcz/libhtml"
	"github.com/alexisbcz/libhtml/secure"
)

func serve(cfg secure.Config) (*httptest.ResponseRecorder, string) {
	var body string
	handler := secure.Headers(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body = h.MustRenderString(secure.Script(r, h.Src("/app.js")))
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	return rec, body
}

func TestHeadersAndNonce(t *testing.T) {
	rec, body := serve(secure.Config{})
	csp := rec.Header().Get("Content-Security-Policy")
	nonce := strings.TrimSuffix(strings.TrimPrefix(body, `<script src="/app.js" nonce="`), `"></script>`)
	if nonce == "" || nonce == body {
		t.Fatalf("no nonce in %q", body)
	}
	if !strings.Contains(csp, "'nonce-"+nonce+"'") || !strings.Contains(csp, "object-src 'none'") || !strings.Contains(csp, "base-uri 'none'") {
		t.Fatalf("csp %q", csp)
	}
	for k, v := range map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "strict-origin-when-cross-origin",
		"Cross-Origin-Opener-Policy": "same-origin",
	} {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q", k, got)
		}
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS on by default")
	}
}

func TestNonceIsFreshPerRequest(t *testing.T) {
	_, a := serve(secure.Config{})
	_, b := serve(secure.Config{})
	if a == b {
		t.Fatal("nonce reused")
	}
}

func TestHSTSAndCustomCSP(t *testing.T) {
	rec, _ := serve(secure.Config{HSTS: 365 * 24 * time.Hour, CSP: func(string) string { return "" }})
	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatal(got)
	}
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("CSP not omitted")
	}
}
