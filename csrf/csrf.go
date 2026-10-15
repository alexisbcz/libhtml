// Package csrf protects state-changing requests against cross-site request
// forgery with two independent layers:
//
//  1. Origin checks: net/http's CrossOriginProtection rejects non-safe
//     requests that the browser marks as cross-site (Sec-Fetch-Site) or whose
//     Origin header does not match the Host.
//  2. Tokens: a signed, HttpOnly, SameSite=Lax "__Host-" cookie holds a
//     random secret, and every non-safe request must echo it back in a form
//     field or header. The echoed token is masked with a fresh one-time pad
//     on every render, so it never appears twice in a response (BREACH).
//
// Wire it up once and drop csrf.Field(r) into each form:
//
//	p, err := csrf.New(csrf.Config{Key: key})
//	http.ListenAndServe(addr, p.Handler(mux))
//
//	h.Form(h.Method("post"), csrf.Field(r), ...)
package csrf

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	h "github.com/alexisbcz/libhtml"
)

const secretLen = 32

// Errors reported by FailureReason.
var (
	ErrCrossOrigin  = errors.New("csrf: cross-origin request")
	ErrNoCookie     = errors.New("csrf: missing or invalid csrf cookie")
	ErrNoToken      = errors.New("csrf: missing csrf token")
	ErrBadToken     = errors.New("csrf: csrf token does not match")
	ErrKeyTooShort  = errors.New("csrf: Config.Key must be at least 32 bytes")
	errNoMiddleware = errors.New("csrf: request did not go through the csrf middleware")
)

// Config configures the protection. Only Key is required.
type Config struct {
	// Key signs the cookie. Use at least 32 random bytes, kept secret and
	// shared by every instance of the app.
	Key []byte

	// CookieName defaults to "__Host-csrf", or "csrf" when Insecure is set.
	// The __Host- prefix makes browsers refuse cookies set by subdomains or
	// over plain HTTP, which blocks cookie-injection attacks.
	CookieName string
	// FieldName is the form field carrying the token. Default "csrf_token".
	FieldName string
	// HeaderName is the request header carrying the token, for fetch/htmx.
	// Default "X-CSRF-Token".
	HeaderName string
	// MaxAge of the cookie. Default 12 hours.
	MaxAge time.Duration

	// Insecure drops the Secure flag and the __Host- prefix so the cookie
	// works over plain HTTP. Only for local development.
	Insecure bool

	// SessionID, if set, binds tokens to the user's session: a token issued
	// for one session is rejected in another. Rotate after login.
	SessionID func(r *http.Request) string

	// TrustedOrigins lists extra origins (e.g. "https://admin.example.com")
	// allowed to make cross-origin non-safe requests.
	TrustedOrigins []string

	// ErrorHandler handles rejected requests. FailureReason(r) tells why.
	// Default: a bare 403.
	ErrorHandler http.Handler
}

// Protector is the configured middleware.
type Protector struct {
	cfg    Config
	origin *http.CrossOriginProtection
}

type ctxKey int

const (
	tokenKey ctxKey = iota
	reasonKey
)

type tokenState struct {
	secret []byte
	field  string
	header string
}

// New validates cfg and returns a Protector.
func New(cfg Config) (*Protector, error) {
	if len(cfg.Key) < 32 {
		return nil, ErrKeyTooShort
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "__Host-csrf"
		if cfg.Insecure {
			cfg.CookieName = "csrf"
		}
	}
	if cfg.FieldName == "" {
		cfg.FieldName = "csrf_token"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-CSRF-Token"
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 12 * time.Hour
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Forbidden - CSRF check failed", http.StatusForbidden)
		})
	}
	origin := http.NewCrossOriginProtection()
	for _, o := range cfg.TrustedOrigins {
		if err := origin.AddTrustedOrigin(o); err != nil {
			return nil, err
		}
	}
	return &Protector{cfg: cfg, origin: origin}, nil
}

// Handler wraps next with CSRF protection.
func (p *Protector) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		secret, ok := p.readCookie(r)
		if !ok {
			secret = make([]byte, secretLen)
			if _, err := rand.Read(secret); err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			p.writeCookie(w, r, secret)
		}
		ctx := context.WithValue(r.Context(), tokenKey, &tokenState{secret: secret, field: p.cfg.FieldName, header: p.cfg.HeaderName})
		r = r.WithContext(ctx)

		if !isSafeMethod(r.Method) {
			if err := p.check(r, secret, ok); err != nil {
				r = r.WithContext(context.WithValue(r.Context(), reasonKey, err))
				p.cfg.ErrorHandler.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (p *Protector) check(r *http.Request, secret []byte, hadCookie bool) error {
	if err := p.origin.Check(r); err != nil {
		return ErrCrossOrigin
	}
	if !hadCookie {
		return ErrNoCookie
	}
	sent := r.Header.Get(p.cfg.HeaderName)
	if sent == "" {
		sent = r.PostFormValue(p.cfg.FieldName)
	}
	if sent == "" {
		return ErrNoToken
	}
	got, ok := unmask(sent)
	if !ok || subtle.ConstantTimeCompare(got, secret) != 1 {
		return ErrBadToken
	}
	return nil
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

// --- cookie ------------------------------------------------------------------

func (p *Protector) mac(r *http.Request, secret []byte) []byte {
	m := hmac.New(sha256.New, p.cfg.Key)
	m.Write([]byte("libhtml-csrf-v1\x00"))
	if p.cfg.SessionID != nil {
		m.Write([]byte(p.cfg.SessionID(r)))
	}
	m.Write([]byte{0})
	m.Write(secret)
	return m.Sum(nil)
}

func (p *Protector) readCookie(r *http.Request) ([]byte, bool) {
	c, err := r.Cookie(p.cfg.CookieName)
	if err != nil {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) != secretLen+sha256.Size {
		return nil, false
	}
	secret, sig := raw[:secretLen], raw[secretLen:]
	if !hmac.Equal(sig, p.mac(r, secret)) {
		return nil, false
	}
	return secret, true
}

func (p *Protector) writeCookie(w http.ResponseWriter, r *http.Request, secret []byte) {
	value := base64.RawURLEncoding.EncodeToString(append(append([]byte{}, secret...), p.mac(r, secret)...))
	http.SetCookie(w, &http.Cookie{
		Name:     p.cfg.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(p.cfg.MaxAge / time.Second),
		Secure:   !p.cfg.Insecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// --- masking -----------------------------------------------------------------

// mask returns base64(pad || pad^secret) with a fresh random pad.
func mask(secret []byte) string {
	out := make([]byte, 2*len(secret))
	pad := out[:len(secret)]
	_, _ = rand.Read(pad) // never fails (crypto/rand panics on failure)
	for i, b := range secret {
		out[len(secret)+i] = pad[i] ^ b
	}
	return base64.RawURLEncoding.EncodeToString(out)
}

func unmask(token string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 2*secretLen {
		return nil, false
	}
	pad, masked := raw[:secretLen], raw[secretLen:]
	out := make([]byte, secretLen)
	for i := range out {
		out[i] = pad[i] ^ masked[i]
	}
	return out, true
}

// --- request helpers ---------------------------------------------------------

func state(r *http.Request) *tokenState {
	s, _ := r.Context().Value(tokenKey).(*tokenState)
	return s
}

// Token returns a fresh masked token for r. Each call returns a different
// string; all of them are valid. It panics if r did not go through the
// middleware, because a form without a valid token would always be rejected.
func Token(r *http.Request) string {
	s := state(r)
	if s == nil {
		panic(errNoMiddleware)
	}
	return mask(s.secret)
}

// Field returns a hidden input carrying the token. Put it in every form that
// submits with a non-safe method.
func Field(r *http.Request) *h.Element {
	s := state(r)
	if s == nil {
		panic(errNoMiddleware)
	}
	return h.Input(h.Type("hidden"), h.Name(s.field), h.Value(mask(s.secret)))
}

// Meta returns <meta name="csrf-token" content="..."> for scripts that send
// the token in the header named by Config.HeaderName (see HeaderName).
func Meta(r *http.Request) *h.Element {
	return h.Meta(h.Name("csrf-token"), h.Content(Token(r)))
}

// HeaderName returns the header name scripts should use for the token.
func HeaderName(r *http.Request) string {
	if s := state(r); s != nil {
		return s.header
	}
	return ""
}

// FailureReason returns why the middleware rejected r, for use in a custom
// ErrorHandler. It is nil for accepted requests.
func FailureReason(r *http.Request) error {
	err, _ := r.Context().Value(reasonKey).(error)
	return err
}
