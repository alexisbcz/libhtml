// Package libhtml builds HTML in Go with a JSX-like API.
//
// Everything can be passed as a child: nodes, strings, numbers, attributes,
// slices, groups and lazy functions. The chained builder methods do the same
// job when you prefer them.
//
//	h.Div(
//		h.Class("p-4"),
//		h.H1("Hello, ", user.Name),
//		h.If(user.Admin, h.A(h.Href("/admin"), "Admin")),
//		h.Ul(h.Map(items, func(it Item) h.Node { return h.Li(it.Title) })),
//	)
//
//	h.Div().Class("p-4").Children(h.H1().Text("Hello, ", user.Name))
//
// Safety is the default. Text and attribute values are always escaped for
// their context, URL attributes are sanitized, and the dangerous sinks
// (event handlers, <script>, <style>, srcdoc, raw HTML) only accept the
// explicitly named Unsafe* types. A reviewer can find every place that
// bypasses a protection by searching for "Unsafe".
package libhtml

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// Node is anything that can render itself as HTML.
type Node interface {
	Render(w io.Writer) error
}

// Elementer is implemented by components that resolve to a single element.
// Slot (and the asChild pattern built on it) uses it to reach that element.
type Elementer interface {
	Element() *Element
}

// NodeFunc adapts a plain function to the Node interface.
type NodeFunc func(w io.Writer) error

// Render calls f(w).
func (f NodeFunc) Render(w io.Writer) error { return f(w) }

// RenderString renders n into a string.
func RenderString(n Node) (string, error) {
	r := getRenderer(nil)
	defer putRenderer(r)
	r.node(n)
	if r.err != nil {
		return "", r.err
	}
	return string(r.buf), nil
}

// MustRenderString is like RenderString but panics on error. It is meant for
// tests and static content known to be valid.
func MustRenderString(n Node) string {
	s, err := RenderString(n)
	if err != nil {
		panic(err)
	}
	return s
}

// WriteHTTP renders n into a buffer and, only if rendering succeeds, writes it
// to w with the given status code. A rendering error therefore never leaves a
// half-written page behind; the caller decides how to report it.
func WriteHTTP(w http.ResponseWriter, status int, n Node) error {
	r := getRenderer(nil)
	defer putRenderer(r)
	r.node(n)
	if r.err != nil {
		return r.err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, err := bytes.NewReader(r.buf).WriteTo(w)
	return err
}

// Handler turns a page function into an http.Handler. Rendering errors are
// reported as a bare 500 without leaking details to the client.
func Handler(page func(r *http.Request) Node) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := WriteHTTP(w, http.StatusOK, page(r)); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
}

// --- basic nodes -----------------------------------------------------------

type textNode string

func (t textNode) renderTo(r *renderer)     { r.escape(string(t)) }
func (t textNode) Render(w io.Writer) error { return render(w, t) }

// Text returns a text node. The content is always HTML-escaped. Multiple
// arguments are concatenated. Plain strings passed as children become text
// nodes automatically.
func Text(s ...string) Node {
	if len(s) == 1 {
		return textNode(s[0])
	}
	return textNode(strings.Join(s, ""))
}

// Fragment groups children without a wrapping element, like <>...</> in JSX.
// It may hold anything an element accepts as a child, including attributes:
// those are applied to the element the fragment is added to.
type Fragment []any

// Group returns a Fragment of the given children.
func Group(children ...any) Fragment { return Fragment(children) }

// Render renders the fragment on its own. Attributes in a fragment that is
// not inside an element are an error.
func (f Fragment) Render(w io.Writer) error { return render(w, f) }

func (f Fragment) renderTo(r *renderer) {
	// Rendering through a throwaway element reuses the child normalization;
	// it has no tag, so only its children are written.
	e := &Element{}
	e.Children(f...)
	if e.err != nil {
		r.fail(e.err)
		return
	}
	if len(e.attrs) > 0 || len(e.classes) > 0 {
		r.fail(&SecurityError{Reason: "attributes in a fragment that is not inside an element"})
		return
	}
	renderChildren(r, e.children)
}

// Nothing renders nothing.
var Nothing Node = Fragment(nil)

type commentNode string

func (c commentNode) renderTo(r *renderer) {
	r.str("<!--")
	r.str(sanitizeComment(string(c)))
	r.str("-->")
}
func (c commentNode) Render(w io.Writer) error { return render(w, c) }

// Comment returns an HTML comment. Sequences that could end the comment early
// are neutralized.
func Comment(s string) Node { return commentNode(s) }

// Doctype renders "<!DOCTYPE html>" followed by its children, usually an
// Html() element.
func Doctype(children ...any) Fragment {
	return append(Fragment{UnsafeHTML("<!DOCTYPE html>")}, children...)
}

// Join returns the children separated by sep.
func Join(sep any, children ...any) Fragment {
	out := make(Fragment, 0, 2*len(children))
	for i, c := range children {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, c)
	}
	return out
}

// Lazy defers building a child until render time.
func Lazy(fn func() Node) Node { return lazyNode(fn) }

type lazyNode func() Node

func (l lazyNode) renderTo(r *renderer)     { r.node(l()) }
func (l lazyNode) Render(w io.Writer) error { return render(w, l) }
