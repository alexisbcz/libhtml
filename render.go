package libhtml

import (
	"io"
	"sync"
)

// renderer buffers output and remembers the first error, so nodes can write
// without checking errors on every call. libhtml's own node types render
// straight into it; foreign Nodes see it as a plain io.Writer.
type renderer struct {
	out  io.Writer // nil when rendering to a string
	buf  []byte
	err  error
	vals []scopedValue // render-scoped values, innermost last (see WithValue)
}

// renderable is the fast path implemented by libhtml's own node types.
type renderable interface {
	renderTo(r *renderer)
}

const flushThreshold = 32 << 10

var rendererPool = sync.Pool{New: func() any { return &renderer{buf: make([]byte, 0, 4096)} }}

func getRenderer(out io.Writer) *renderer {
	r := rendererPool.Get().(*renderer)
	r.out, r.err = out, nil
	return r
}

func putRenderer(r *renderer) {
	if cap(r.buf) > 1<<20 {
		return // don't keep huge buffers alive
	}
	r.out, r.err, r.buf, r.vals = nil, nil, r.buf[:0], r.vals[:0]
	rendererPool.Put(r)
}

// render is the entry point used by every public Render method.
func render(w io.Writer, n renderable) error {
	if r, ok := w.(*renderer); ok {
		n.renderTo(r)
		return r.err
	}
	r := getRenderer(w)
	n.renderTo(r)
	r.flush()
	err := r.err
	putRenderer(r)
	return err
}

func (r *renderer) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *renderer) flush() {
	if r.out == nil || len(r.buf) == 0 || r.err != nil {
		return
	}
	if _, err := r.out.Write(r.buf); err != nil {
		r.fail(err)
	}
	r.buf = r.buf[:0]
}

func (r *renderer) maybeFlush() {
	if r.out != nil && len(r.buf) >= flushThreshold {
		r.flush()
	}
}

// Write implements io.Writer for foreign Nodes.
func (r *renderer) Write(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	r.buf = append(r.buf, p...)
	r.maybeFlush()
	return len(p), r.err
}

// WriteString implements io.StringWriter so io.WriteString avoids a copy.
func (r *renderer) WriteString(s string) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	r.buf = append(r.buf, s...)
	r.maybeFlush()
	return len(s), r.err
}

func (r *renderer) str(s string) { r.buf = append(r.buf, s...) }

func (r *renderer) byte(c byte) { r.buf = append(r.buf, c) }

// node renders any Node, using the fast path when available.
func (r *renderer) node(n Node) {
	if r.err != nil {
		return
	}
	switch v := n.(type) {
	case nil:
	case renderable:
		v.renderTo(r)
	default:
		if err := n.Render(r); err != nil {
			r.fail(err)
		}
	}
	r.maybeFlush()
}

// escape appends s escaped for text or double-quoted attribute context,
// copying unescaped runs in one go and allocating nothing.
func (r *renderer) escape(s string) {
	last := 0
	for i := 0; i < len(s); i++ {
		var esc string
		switch s[i] {
		case '&':
			esc = "&amp;"
		case '<':
			esc = "&lt;"
		case '>':
			esc = "&gt;"
		case '"':
			esc = "&#34;"
		case '\'':
			esc = "&#39;"
		case 0:
			esc = "�"
		default:
			continue
		}
		r.buf = append(r.buf, s[last:i]...)
		r.buf = append(r.buf, esc...)
		last = i + 1
	}
	r.buf = append(r.buf, s[last:]...)
}

// --- render-scoped values -------------------------------------------------------

type scopedValue struct{ key, val any }

type scopeNode struct {
	key, val any
	children Fragment
}

// WithValue renders children with key set to val, like a React context
// provider: components below read it with Lookup while rendering. Typical use
// is choosing a visual style for part of a page.
func WithValue(key, val any, children ...any) Node {
	return scopeNode{key: key, val: val, children: Fragment(children)}
}

func (s scopeNode) Render(w io.Writer) error { return render(w, s) }

func (s scopeNode) renderTo(r *renderer) {
	r.vals = append(r.vals, scopedValue{s.key, s.val})
	s.children.renderTo(r)
	r.vals = r.vals[:len(r.vals)-1]
}

// Lookup returns the innermost value set for key by an enclosing WithValue,
// when w is the writer libhtml passes to a Node's Render method.
func Lookup(w io.Writer, key any) (any, bool) {
	r, ok := w.(*renderer)
	if !ok {
		return nil, false
	}
	for i := len(r.vals) - 1; i >= 0; i-- {
		if r.vals[i].key == key {
			return r.vals[i].val, true
		}
	}
	return nil, false
}

// Children returns the children of a WithValue node, so code that walks a
// component tree before rendering can descend into it.
func (s scopeNode) Children() []any { return s.children }

// Scoped is implemented by WithValue nodes: they render their Children in a
// scope. Tree walkers use it to descend into them.
type Scoped interface {
	Node
	Children() []any
}
