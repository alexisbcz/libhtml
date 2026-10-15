package csrf_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	h "github.com/alexisbcz/libhtml"
	"github.com/alexisbcz/libhtml/csrf"
)

var key = bytes.Repeat([]byte("k"), 32)

func newServer(t *testing.T, cfg csrf.Config) (http.Handler, *error) {
	t.Helper()
	if cfg.Key == nil {
		cfg.Key = key
	}
	var reason error
	cfg.ErrorHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reason = csrf.FailureReason(r)
		w.WriteHeader(http.StatusForbidden)
	})
	p, err := csrf.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /form", func(w http.ResponseWriter, r *http.Request) {
		_ = h.WriteHTTP(w, 200, h.Form(h.Method("post"), csrf.Field(r)))
	})
	mux.HandleFunc("POST /form", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return p.Handler(mux), &reason
}

var tokenRe = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

// getForm fetches the form and returns its cookie and token.
func getForm(t *testing.T, srv http.Handler, cookies ...*http.Cookie) (*http.Cookie, string) {
	t.Helper()
	req := httptest.NewRequest("GET", "https://example.com/form", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	m := tokenRe.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatalf("no token in %q", rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		cookie = c
	}
	if cookie == nil && len(cookies) > 0 {
		cookie = cookies[0]
	}
	return cookie, m[1]
}

func post(srv http.Handler, cookie *http.Cookie, form url.Values, hdr map[string]string) int {
	req := httptest.NewRequest("POST", "https://example.com/form", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Code
}

func TestKeyRequired(t *testing.T) {
	if _, err := csrf.New(csrf.Config{Key: []byte("short")}); !errors.Is(err, csrf.ErrKeyTooShort) {
		t.Fatal(err)
	}
}

func TestCookieAttributes(t *testing.T) {
	srv, _ := newServer(t, csrf.Config{})
	c, _ := getForm(t, srv)
	if c.Name != "__Host-csrf" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("weak cookie: %+v", c)
	}
}

func TestValidSubmission(t *testing.T) {
	srv, _ := newServer(t, csrf.Config{})
	c, tok := getForm(t, srv)
	if code := post(srv, c, url.Values{"csrf_token": {tok}}, nil); code != http.StatusNoContent {
		t.Fatalf("form token rejected: %d", code)
	}
	if code := post(srv, c, nil, map[string]string{"X-CSRF-Token": tok}); code != http.StatusNoContent {
		t.Fatalf("header token rejected: %d", code)
	}
}

func TestTokensAreMaskedPerRender(t *testing.T) {
	srv, _ := newServer(t, csrf.Config{})
	c, tok1 := getForm(t, srv)
	_, tok2 := getForm(t, srv, c)
	if tok1 == tok2 {
		t.Fatal("token repeated across renders")
	}
	if code := post(srv, c, url.Values{"csrf_token": {tok1}}, nil); code != http.StatusNoContent {
		t.Fatal("first token invalid")
	}
	if code := post(srv, c, url.Values{"csrf_token": {tok2}}, nil); code != http.StatusNoContent {
		t.Fatal("second token invalid")
	}
}

func TestRejections(t *testing.T) {
	srv, reason := newServer(t, csrf.Config{})
	c, tok := getForm(t, srv)
	_, otherTok := getForm(t, srv)

	cases := []struct {
		name   string
		cookie *http.Cookie
		form   url.Values
		hdr    map[string]string
		want   error
	}{
		{"no cookie", nil, url.Values{"csrf_token": {tok}}, nil, csrf.ErrNoCookie},
		{"no token", c, nil, nil, csrf.ErrNoToken},
		{"garbage token", c, url.Values{"csrf_token": {"abc"}}, nil, csrf.ErrBadToken},
		{"token from another browser", c, url.Values{"csrf_token": {otherTok}}, nil, csrf.ErrBadToken},
		{"forged cookie", &http.Cookie{Name: "__Host-csrf", Value: strings.Repeat("A", 86)}, url.Values{"csrf_token": {tok}}, nil, csrf.ErrNoCookie},
		{"cross-site fetch", c, url.Values{"csrf_token": {tok}}, map[string]string{"Sec-Fetch-Site": "cross-site"}, csrf.ErrCrossOrigin},
		{"foreign origin", c, url.Values{"csrf_token": {tok}}, map[string]string{"Sec-Fetch-Site": "", "Origin": "https://evil.example"}, csrf.ErrCrossOrigin},
	}
	for _, tc := range cases {
		*reason = nil
		if code := post(srv, tc.cookie, tc.form, tc.hdr); code != http.StatusForbidden {
			t.Errorf("%s: got %d, want 403", tc.name, code)
			continue
		}
		if !errors.Is(*reason, tc.want) {
			t.Errorf("%s: reason %v, want %v", tc.name, *reason, tc.want)
		}
	}
}

func TestCookieSignedWithKey(t *testing.T) {
	srv, _ := newServer(t, csrf.Config{})
	c, tok := getForm(t, srv)
	other, _ := newServer(t, csrf.Config{Key: bytes.Repeat([]byte("z"), 32)})
	if code := post(other, c, url.Values{"csrf_token": {tok}}, nil); code != http.StatusForbidden {
		t.Fatal("cookie signed with another key accepted")
	}
}

func TestSessionBinding(t *testing.T) {
	session := "alice"
	srv, reason := newServer(t, csrf.Config{SessionID: func(*http.Request) string { return session }})
	c, tok := getForm(t, srv)
	if code := post(srv, c, url.Values{"csrf_token": {tok}}, nil); code != http.StatusNoContent {
		t.Fatal("same session rejected")
	}
	session = "mallory"
	if code := post(srv, c, url.Values{"csrf_token": {tok}}, nil); code != http.StatusForbidden || !errors.Is(*reason, csrf.ErrNoCookie) {
		t.Fatalf("token reused across sessions: %d %v", code, *reason)
	}
}

func TestSafeMethodsPass(t *testing.T) {
	srv, _ := newServer(t, csrf.Config{})
	req := httptest.NewRequest("GET", "https://example.com/form", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}

func TestInsecureMode(t *testing.T) {
	srv, _ := newServer(t, csrf.Config{Insecure: true})
	c, _ := getForm(t, srv)
	if c.Name != "csrf" || c.Secure {
		t.Fatalf("%+v", c)
	}
}
