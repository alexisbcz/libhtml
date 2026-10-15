package libhtml

import (
	"strings"
)

// FilteredValue replaces a URL or style value that failed sanitization. It is
// harmless in every context and easy to spot when debugging a page.
const FilteredValue = "about:invalid#libhtml-filtered"

// filteredStyle replaces a style declaration list that failed sanitization.
const filteredStyle = "/* libhtml-filtered */"

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&#34;",
	"'", "&#39;",
	"\x00", "�",
)

// EscapeText escapes s for use as HTML text content.
func EscapeText(s string) string { return htmlEscaper.Replace(s) }

// EscapeAttr escapes s for use inside a double-quoted attribute value.
// Attribute values are always quoted by libhtml, so this is sufficient.
func EscapeAttr(s string) string { return htmlEscaper.Replace(s) }

// --- names -----------------------------------------------------------------

// validTagName accepts HTML, SVG/MathML (camelCase, e.g. foreignObject) and
// custom-element (hyphenated) names.
func validTagName(tag string) bool {
	if tag == "" || !isASCIILetter(tag[0]) {
		return false
	}
	for i := 1; i < len(tag); i++ {
		c := tag[i]
		if !isASCIILetter(c) && !isASCIIDigit(c) && c != '-' {
			return false
		}
	}
	return true
}

// validAttrName accepts the attribute names a browser parses as a single
// attribute, plus ':', '@' and '.' used by Alpine, Vue and htmx. Anything that
// could break out of the attribute list (spaces, quotes, '=', '<', '>', '/')
// is rejected.
func validAttrName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case isASCIILetter(c), isASCIIDigit(c):
		case c == '-' || c == '_' || c == ':' || c == '.' || c == '@':
		default:
			return false
		}
	}
	return true
}

func isASCIILetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isASCIIDigit(c byte) bool  { return c >= '0' && c <= '9' }

// --- attribute classification ----------------------------------------------

type attrKind int

const (
	kindPlain attrKind = iota
	kindURL
	kindSrcset
	kindStyle
	kindJS   // event handlers and framework expression attributes
	kindHTML // srcdoc: a whole HTML document in an attribute
)

var urlAttrs = map[string]bool{
	"action": true, "background": true, "cite": true, "codebase": true,
	"data": true, "formaction": true, "href": true, "icon": true,
	"longdesc": true, "manifest": true, "ping": true, "poster": true,
	"profile": true, "src": true, "usemap": true, "xlink:href": true,
	"xml:base": true, "hx-get": true, "hx-post": true, "hx-put": true,
	"hx-patch": true, "hx-delete": true, "hx-push-url": true,
	"hx-replace-url": true,
}

// classifyAttr decides which protection applies to an attribute value.
func classifyAttr(name string) attrKind {
	n := strings.ToLower(name)
	switch {
	case urlAttrs[n]:
		return kindURL
	case n == "srcset" || n == "imagesrcset":
		return kindSrcset
	case n == "style":
		return kindStyle
	case n == "srcdoc":
		return kindHTML
	case len(n) > 2 && strings.HasPrefix(n, "on"):
		return kindJS
	// Framework attributes that evaluate their value as JavaScript.
	case strings.HasPrefix(n, "@"), strings.HasPrefix(n, ":"),
		strings.HasPrefix(n, "x-"), strings.HasPrefix(n, "v-"),
		strings.HasPrefix(n, "hx-on"), strings.HasPrefix(n, "data-hx-on"),
		strings.HasPrefix(n, "_"), n == "hx-vals", n == "hx-vars":
		return kindJS
	}
	return kindPlain
}

// --- URLs ------------------------------------------------------------------

var safeSchemes = map[string]bool{"http": true, "https": true, "mailto": true, "tel": true}

// SanitizeURL returns u if it is a relative URL or uses one of the http,
// https, mailto or tel schemes, and FilteredValue otherwise. It is what every
// URL attribute goes through unless the value is an UnsafeURL.
func SanitizeURL(u string) string {
	if isSafeURL(u, false) {
		return u
	}
	return FilteredValue
}

// isSafeURL reports whether u is safe in a URL attribute. With allowImageData
// it also accepts data: URLs for raster image types (never SVG, which can
// carry script).
func isSafeURL(u string, allowImageData bool) bool {
	// Browsers drop leading/trailing C0 controls and spaces and remove tabs
	// and newlines anywhere before resolving a URL, so "java\tscript:" is a
	// javascript: URL. Normalize the same way before looking at the scheme.
	n := u
	for i := 0; i < len(u); i++ {
		if c := u[i]; c <= 0x20 || c == 0x7f {
			n = stripControls(u)
			break
		}
	}

	colon := strings.IndexByte(n, ':')
	if colon < 0 {
		return true // no scheme: relative URL
	}
	if i := strings.IndexAny(n, "/?#"); i >= 0 && i < colon {
		return true // the colon is in the path, query or fragment
	}
	scheme := strings.ToLower(n[:colon])
	if safeSchemes[scheme] {
		return true
	}
	if allowImageData && scheme == "data" {
		rest := strings.ToLower(n[colon+1:])
		for _, t := range []string{"image/png", "image/gif", "image/jpeg", "image/jpg", "image/webp", "image/avif", "image/bmp"} {
			if strings.HasPrefix(rest, t+";") || strings.HasPrefix(rest, t+",") {
				return true
			}
		}
	}
	return false
}

func stripControls(u string) string {
	b := make([]byte, 0, len(u))
	for i := 0; i < len(u); i++ {
		if c := u[i]; c > 0x20 && c != 0x7f {
			b = append(b, c)
		}
	}
	return string(b)
}

// sanitizeSrcset checks each candidate URL of a srcset value.
func sanitizeSrcset(v string) string {
	for _, cand := range strings.Split(v, ",") {
		fields := strings.Fields(cand)
		if len(fields) == 0 {
			continue
		}
		if !isSafeURL(fields[0], true) {
			return FilteredValue
		}
	}
	return v
}

// --- CSS -------------------------------------------------------------------

// sanitizeStyle accepts an inline style unless it contains constructs that can
// run script, load content from an unchecked URL, or break out of the value.
func sanitizeStyle(v string) string {
	l := strings.ToLower(v)
	if strings.ContainsAny(l, `<>\"`) {
		return filteredStyle
	}
	for _, bad := range []string{"expression", "javascript:", "vbscript:", "-moz-binding", "behavior", "@import", "</"} {
		if strings.Contains(l, bad) {
			return filteredStyle
		}
	}
	// Every url(...) must point at a safe URL.
	for rest := l; ; {
		i := strings.Index(rest, "url(")
		if i < 0 {
			break
		}
		rest = rest[i+4:]
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			return filteredStyle
		}
		target := strings.Trim(strings.TrimSpace(rest[:end]), `'`)
		if !isSafeURL(target, true) {
			return filteredStyle
		}
		rest = rest[end+1:]
	}
	return v
}

// --- comments --------------------------------------------------------------

func sanitizeComment(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "- -")
	}
	s = strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(s)
	return strings.TrimSuffix(s, "-")
}
