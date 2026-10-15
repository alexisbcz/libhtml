package libhtml

import (
	"io"
)

// The Unsafe* types mark content that libhtml will not escape or sanitize in
// the context it is meant for. Never build them from user input. Converting a
// string to one of these types is the one way to opt out of a protection, so
// every bypass is easy to find in code review.

// UnsafeHTML is markup that is written to the output as-is.
type UnsafeHTML string

// Render writes the markup without escaping.
func (u UnsafeHTML) Render(w io.Writer) error { return render(w, u) }
func (u UnsafeHTML) renderTo(r *renderer)     { r.str(string(u)) }

// UnsafeJS is JavaScript source. It is accepted as the content of a <script>
// element and as the value of event-handler attributes (see Element.On and
// Element.JSAttr). Anywhere else it is escaped like ordinary text.
type UnsafeJS string

// Render escapes the source as text. Inside a <script> element it is written
// raw instead (after checking it cannot close the element early).
func (u UnsafeJS) Render(w io.Writer) error { return render(w, textNode(u)) }
func (u UnsafeJS) renderTo(r *renderer)     { r.escape(string(u)) }

// UnsafeCSS is a stylesheet. It is accepted as the content of a <style>
// element. Anywhere else it is escaped like text.
type UnsafeCSS string

// Render escapes the stylesheet as text. Inside a <style> element it is
// written raw instead (after checking it cannot close the element early).
func (u UnsafeCSS) Render(w io.Writer) error { return render(w, textNode(u)) }
func (u UnsafeCSS) renderTo(r *renderer)     { r.escape(string(u)) }

// UnsafeURL is a URL that skips scheme sanitization, for example a data: URL
// you produced yourself. It is still attribute-escaped.
type UnsafeURL string
