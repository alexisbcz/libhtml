// Package secure sets browser security headers, including a strict
// nonce-based Content-Security-Policy, so that even markup that slips past
// libhtml's escaping cannot run injected scripts.
//
//	http.ListenAndServe(addr, secure.Headers(secure.Config{})(mux))
//
//	h.Head(secure.Script(r, h.Src("/app.js")))
package secure

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	h "github.com/alexisbcz/libhtml"
)

// Config configures the headers. The zero value is a strict, sensible
// default.
type Config struct {
	// CSP builds the Content-Security-Policy for a request's nonce. Default:
	// DefaultCSP. Return "" to omit the header.
	CSP func(nonce string) string
	// ReferrerPolicy. Default "strict-origin-when-cross-origin".
	ReferrerPolicy string
	// FrameOptions. Default "DENY". The default CSP also sets
	// frame-ancestors 'none'.
	FrameOptions string
	// CrossOriginOpenerPolicy. Default "same-origin".
	CrossOriginOpenerPolicy string
	// PermissionsPolicy, e.g. "camera=(), microphone=()". Default unset.
	PermissionsPolicy string
	// HSTS sets Strict-Transport-Security with this max-age. Only enable it
	// once the site is served exclusively over HTTPS. Default off.
	HSTS time.Duration
}

// DefaultCSP is a strict policy: scripts run only with the request's nonce
// (and what they load, via 'strict-dynamic'), there are no plugins, no
// <base> hijacking, no framing and forms only post to the same origin.
// Inline style attributes are allowed (style-src-attr): components render
// them (progress transforms, chart colors, CSS variables) and libhtml
// sanitizes their values; <style> elements still need the nonce.
func DefaultCSP(nonce string) string {
	n := "'nonce-" + nonce + "'"
	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + n + " 'strict-dynamic'",
		"style-src 'self' " + n,
		"style-src-attr 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self'",
		"object-src 'none'",
		"base-uri 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
	}, "; ")
}

type ctxKey struct{}

// Headers returns middleware that sets the security headers and a fresh CSP
// nonce on every request.
func Headers(cfg Config) func(http.Handler) http.Handler {
	if cfg.CSP == nil {
		cfg.CSP = DefaultCSP
	}
	if cfg.ReferrerPolicy == "" {
		cfg.ReferrerPolicy = "strict-origin-when-cross-origin"
	}
	if cfg.FrameOptions == "" {
		cfg.FrameOptions = "DENY"
	}
	if cfg.CrossOriginOpenerPolicy == "" {
		cfg.CrossOriginOpenerPolicy = "same-origin"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce := newNonce()
			hdr := w.Header()
			if csp := cfg.CSP(nonce); csp != "" {
				hdr.Set("Content-Security-Policy", csp)
			}
			hdr.Set("X-Content-Type-Options", "nosniff")
			hdr.Set("Referrer-Policy", cfg.ReferrerPolicy)
			hdr.Set("X-Frame-Options", cfg.FrameOptions)
			hdr.Set("Cross-Origin-Opener-Policy", cfg.CrossOriginOpenerPolicy)
			if cfg.PermissionsPolicy != "" {
				hdr.Set("Permissions-Policy", cfg.PermissionsPolicy)
			}
			if cfg.HSTS > 0 {
				hdr.Set("Strict-Transport-Security", "max-age="+strconv.Itoa(int(cfg.HSTS/time.Second))+"; includeSubDomains")
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, nonce)))
		})
	}
}

func newNonce() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b) // never fails (crypto/rand panics on failure)
	return base64.RawStdEncoding.EncodeToString(b)
}

// Nonce returns the CSP nonce for r, or "" if r did not go through Headers.
func Nonce(r *http.Request) string {
	n, _ := r.Context().Value(ctxKey{}).(string)
	return n
}

// Script returns a <script> element carrying the request's nonce.
func Script(r *http.Request, children ...any) *h.Element {
	return h.Script(children...).Nonce(Nonce(r))
}

// Style returns a <style> element carrying the request's nonce.
func Style(r *http.Request, children ...any) *h.Element {
	return h.StyleEl(children...).Nonce(Nonce(r))
}

// Link returns a <link> element carrying the request's nonce, for
// stylesheets under a nonce-based style-src.
func Link(r *http.Request, children ...any) *h.Element {
	return h.Link(children...).Nonce(Nonce(r))
}
