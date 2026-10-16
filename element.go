package libhtml

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// SecurityError reports markup that libhtml refuses to render because it
// would open an injection hole, such as an event handler set from a plain
// string or a <script> element with text content.
type SecurityError struct {
	Tag    string
	Attr   string
	Reason string
}

func (e *SecurityError) Error() string {
	var b strings.Builder
	b.WriteString("libhtml: refused to render")
	if e.Tag != "" {
		b.WriteString(" <" + e.Tag + ">")
	}
	if e.Attr != "" {
		b.WriteString(" attribute " + strconv.Quote(e.Attr))
	}
	b.WriteString(": " + e.Reason)
	return b.String()
}

// IsSecurityError reports whether err is (or wraps) a *SecurityError.
func IsSecurityError(err error) bool {
	var se *SecurityError
	return errors.As(err, &se)
}

type attr struct {
	name    string
	value   string
	boolean bool // rendered as a bare name, e.g. disabled
	trusted bool // set through an Unsafe* API: skips value sanitization
}

// Element is an HTML element under construction. Every builder method
// mutates the element and returns it, so calls can be chained. Use Clone to
// branch off a shared base element.
type Element struct {
	tag      string
	attrs    []attr
	classes  []string
	children []any // each a string (text) or a Node
	err      error
}

// El returns a new element with the given tag. Prefer the generated helpers
// (Div, Span, A, ...) for standard tags. See Element.Children for what the
// children may be.
func El(tag string, children ...any) *Element {
	e := &Element{tag: tag}
	if len(children) > 0 {
		e.children = make([]any, 0, len(children))
		e.Children(children...)
	}
	return e
}

// Element returns e itself, so *Element satisfies Elementer.
func (e *Element) Element() *Element { return e }

// --- attributes --------------------------------------------------------------

func (e *Element) fail(name, reason string) *Element {
	if e.err == nil {
		e.err = &SecurityError{Tag: e.tag, Attr: name, Reason: reason}
	}
	return e
}

func (e *Element) index(name string) int {
	for i, a := range e.attrs {
		if strings.EqualFold(a.name, name) {
			return i
		}
	}
	return -1
}

func (e *Element) set(a attr) *Element {
	if !validAttrName(a.name) {
		return e.fail(a.name, "invalid attribute name")
	}
	if strings.EqualFold(a.name, "class") {
		e.SetClass(a.value)
		return e
	}
	if i := e.index(a.name); i >= 0 {
		e.attrs[i] = a
	} else {
		e.attrs = append(e.attrs, a)
	}
	return e
}

// Attr sets an attribute, replacing any previous value. The value is always
// escaped. URL attributes (href, src, action, ...) are sanitized, style is
// checked for script-capable CSS, and attributes that execute JavaScript
// (on*, x-*, @*, hx-on*, ...) or contain HTML (srcdoc) are refused: use On,
// JSAttr or UnsafeAttr for those.
func (e *Element) Attr(name, value string) *Element {
	return e.set(attr{name: name, value: value})
}

// AttrIf sets the attribute only when cond is true.
func (e *Element) AttrIf(cond bool, name, value string) *Element {
	if cond {
		return e.Attr(name, value)
	}
	return e
}

// Attrs sets several attributes, in sorted key order for deterministic output.
func (e *Element) Attrs(m map[string]string) *Element {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e.Attr(k, m[k])
	}
	return e
}

// UnsafeAttr sets an attribute whose value skips sanitization. The value is
// still escaped, so it cannot break out of the attribute, but a javascript:
// URL or event handler built from user input would run.
func (e *Element) UnsafeAttr(name, value string) *Element {
	return e.set(attr{name: name, value: value, trusted: true})
}

// UnsafeURL sets a URL attribute without scheme sanitization.
func (e *Element) UnsafeURL(name string, u UnsafeURL) *Element {
	return e.UnsafeAttr(name, string(u))
}

// Bool adds (on=true) or removes (on=false) a boolean attribute such as
// disabled or required.
func (e *Element) Bool(name string, on bool) *Element {
	if !on {
		return e.Remove(name)
	}
	return e.set(attr{name: name, boolean: true})
}

// Remove deletes an attribute (or all classes, for "class").
func (e *Element) Remove(name string) *Element {
	if strings.EqualFold(name, "class") {
		e.classes = nil
		return e
	}
	if i := e.index(name); i >= 0 {
		e.attrs = slices.Delete(e.attrs, i, i+1)
	}
	return e
}

// On sets an inline event handler, e.g. On("click", "toggle()"). The event
// name is the part after "on".
func (e *Element) On(event string, js UnsafeJS) *Element {
	for i := 0; i < len(event); i++ {
		if !isASCIILetter(event[i]) {
			return e.fail("on"+event, "invalid event name")
		}
	}
	return e.set(attr{name: "on" + strings.ToLower(event), value: string(js), trusted: true})
}

// JSAttr sets an attribute whose value is evaluated as JavaScript by a
// framework, such as Alpine's x-data or htmx's hx-on:click.
func (e *Element) JSAttr(name string, js UnsafeJS) *Element {
	return e.set(attr{name: name, value: string(js), trusted: true})
}

// --- classes -------------------------------------------------------------------

// Class appends classes. Each argument may hold several space-separated
// classes.
func (e *Element) Class(classes ...string) *Element {
	for _, c := range classes {
		if !isBlank(c) {
			e.classes = append(e.classes, c)
		}
	}
	return e
}

// ClassIf appends classes only when cond is true.
func (e *Element) ClassIf(cond bool, classes ...string) *Element {
	if cond {
		return e.Class(classes...)
	}
	return e
}

// SetClass replaces all classes.
func (e *Element) SetClass(classes string) *Element {
	e.classes = e.classes[:0]
	return e.Class(classes)
}

// ClassName returns the classes as a single space-separated string.
func (e *Element) ClassName() string {
	if len(e.classes) == 1 && isNormalized(e.classes[0]) {
		return e.classes[0]
	}
	return string(appendClasses(nil, e.classes))
}

// Class strings are stored as given and normalized (single spaces, no
// leading or trailing space) only when written, which avoids splitting and
// re-joining the long class lists components pass around.

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

func isBlank(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isSpace(s[i]) {
			return false
		}
	}
	return true
}

func isNormalized(s string) bool {
	if s == "" || isSpace(s[0]) || isSpace(s[len(s)-1]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if isSpace(s[i]) && (s[i] != ' ' || s[i-1] == ' ') {
			return false
		}
	}
	return true
}

// appendClasses appends the space-separated tokens of all class strings.
func appendClasses(dst []byte, classes []string) []byte {
	first := true
	for _, c := range classes {
		for i := 0; i < len(c); {
			for i < len(c) && isSpace(c[i]) {
				i++
			}
			start := i
			for i < len(c) && !isSpace(c[i]) {
				i++
			}
			if start < i {
				if !first {
					dst = append(dst, ' ')
				}
				dst = append(dst, c[start:i]...)
				first = false
			}
		}
	}
	return dst
}

// --- children ----------------------------------------------------------------

// Children appends children. Each argument may be:
//
//   - a Node (elements, components, Text, UnsafeHTML, ...)
//   - a Prop (Class, ID, Href, Attr, ...), applied to this element
//   - a string, escaped as text; a number or bool, formatted as text
//   - a Fragment, []Node, []any, []string or []*Element, flattened
//   - a func() Node, called at render time
//   - nil (including a nil *Element), skipped
//
// Anything else is recorded as an error and reported by Render.
func (e *Element) Children(children ...any) *Element {
	for _, c := range children {
		e.add(c)
	}
	return e
}

func (e *Element) add(c any) {
	switch v := c.(type) {
	case nil:
	case string:
		e.children = append(e.children, c) // reuse the boxed string
	case *Element:
		if v != nil {
			e.children = append(e.children, v)
		}
	case Prop:
		if v != nil {
			v(e)
		}
	case Fragment:
		for _, x := range v {
			e.add(x)
		}
	case interface{ Fragment() Fragment }:
		e.add(v.Fragment())
	case Node:
		if !IsNil(v) {
			e.children = append(e.children, v)
		}
	case []Node:
		for _, x := range v {
			e.add(x)
		}
	case []any:
		for _, x := range v {
			e.add(x)
		}
	case []string:
		for _, x := range v {
			e.children = append(e.children, textNode(x))
		}
	case []*Element:
		for _, x := range v {
			e.add(x)
		}
	case []Prop:
		for _, x := range v {
			e.add(x)
		}
	case func() Node:
		e.children = append(e.children, lazyNode(v))
	case int:
		e.children = append(e.children, textNode(strconv.Itoa(v)))
	case int64:
		e.children = append(e.children, textNode(strconv.FormatInt(v, 10)))
	case int32:
		e.children = append(e.children, textNode(strconv.FormatInt(int64(v), 10)))
	case uint:
		e.children = append(e.children, textNode(strconv.FormatUint(uint64(v), 10)))
	case uint64:
		e.children = append(e.children, textNode(strconv.FormatUint(v, 10)))
	case uint32:
		e.children = append(e.children, textNode(strconv.FormatUint(uint64(v), 10)))
	case float64:
		e.children = append(e.children, textNode(strconv.FormatFloat(v, 'g', -1, 64)))
	case float32:
		e.children = append(e.children, textNode(strconv.FormatFloat(float64(v), 'g', -1, 32)))
	case bool:
		e.children = append(e.children, textNode(strconv.FormatBool(v)))
	case fmt.Stringer:
		e.children = append(e.children, textNode(v.String()))
	default:
		if e.err == nil {
			e.err = fmt.Errorf("libhtml: <%s>: unsupported child of type %T", e.tag, c)
		}
	}
}

// Text appends escaped text. Multiple arguments are concatenated.
func (e *Element) Text(s ...string) *Element {
	e.children = append(e.children, Text(s...))
	return e
}

// Textf appends escaped, fmt-formatted text.
func (e *Element) Textf(format string, args ...any) *Element {
	e.children = append(e.children, textNode(fmt.Sprintf(format, args...)))
	return e
}

// SetChildren replaces all children.
func (e *Element) SetChildren(children ...any) *Element {
	e.children = nil
	return e.Children(children...)
}

// --- composition -------------------------------------------------------------

// With applies fn to the element, for reusable groups of builder calls.
func (e *Element) With(fn func(*Element)) *Element {
	fn(e)
	return e
}

// When applies fn only when cond is true.
func (e *Element) When(cond bool, fn func(*Element)) *Element {
	if cond {
		fn(e)
	}
	return e
}

// Clone returns a copy that can be modified without affecting e. Children are
// shared, not deep-copied.
func (e *Element) Clone() *Element {
	if e == nil {
		return nil
	}
	return &Element{
		tag:      e.tag,
		attrs:    slices.Clone(e.attrs),
		classes:  slices.Clone(e.classes),
		children: slices.Clone(e.children),
		err:      e.err,
	}
}

// --- inspection --------------------------------------------------------------

// Tag returns the element's tag name.
func (e *Element) Tag() string { return e.tag }

// GetAttr returns an attribute's value and whether it is set. Boolean
// attributes report an empty value.
func (e *Element) GetAttr(name string) (string, bool) {
	if strings.EqualFold(name, "class") {
		return e.ClassName(), len(e.classes) > 0
	}
	if i := e.index(name); i >= 0 {
		return e.attrs[i].value, true
	}
	return "", false
}

// Has reports whether an attribute is set.
func (e *Element) Has(name string) bool {
	_, ok := e.GetAttr(name)
	return ok
}

// ChildNodes returns the element's children, each a string (text) or a
// Node. The slice must not be modified.
func (e *Element) ChildNodes() []any { return e.children }

// Err returns the first error recorded while building the element, if any.
func (e *Element) Err() error { return e.err }

// --- rendering ---------------------------------------------------------------

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"source": true, "track": true, "wbr": true,
}

// IsVoid reports whether tag is an HTML void element (no children, no end
// tag).
func IsVoid(tag string) bool { return voidElements[strings.ToLower(tag)] }

// Render writes the element and its children to w.
func (e *Element) Render(w io.Writer) error {
	if e == nil {
		return nil
	}
	return render(w, e)
}

func (e *Element) renderTo(r *renderer) {
	if e == nil || r.err != nil {
		return
	}
	if e.err != nil {
		r.fail(e.err)
		return
	}
	if !validTagName(e.tag) {
		r.fail(&SecurityError{Tag: e.tag, Reason: "invalid tag name"})
		return
	}
	tag := e.tag
	if !isLowerASCII(tag) {
		tag = strings.ToLower(tag)
	}
	if tag == "plaintext" || tag == "xmp" {
		r.fail(&SecurityError{Tag: e.tag, Reason: "obsolete raw-text element is not supported"})
		return
	}
	void := voidElements[tag]
	if void && len(e.children) > 0 {
		r.fail(&SecurityError{Tag: e.tag, Reason: "void element cannot have children"})
		return
	}

	// Validate before writing so a refused element leaves no partial tag.
	attrs := e.renderAttrs(tag)
	mark := len(r.buf)
	r.byte('<')
	r.str(e.tag)
	if len(e.classes) > 0 {
		r.str(` class="`)
		if len(e.classes) == 1 && isNormalized(e.classes[0]) {
			r.escape(e.classes[0])
		} else {
			r.escape(string(appendClasses(nil, e.classes)))
		}
		r.byte('"')
	}
	for _, a := range attrs {
		if !validAttrName(a.name) {
			r.buf = r.buf[:mark]
			r.fail(&SecurityError{Tag: e.tag, Attr: a.name, Reason: "invalid attribute name"})
			return
		}
		r.byte(' ')
		r.str(a.name)
		if a.boolean {
			continue
		}
		v, err := e.attrValue(tag, a)
		if err != nil {
			r.buf = r.buf[:mark]
			r.fail(err)
			return
		}
		r.str(`="`)
		r.escape(v)
		r.byte('"')
	}
	r.byte('>')
	if void {
		return
	}

	switch tag {
	case "script":
		e.renderRawText(r, tag, func(n any) (string, bool) {
			js, ok := n.(UnsafeJS)
			return string(js), ok
		})
	case "style":
		e.renderRawText(r, tag, func(n any) (string, bool) {
			css, ok := n.(UnsafeCSS)
			return string(css), ok
		})
	default:
		renderChildren(r, e.children)
	}
	r.str("</")
	r.str(e.tag)
	r.byte('>')
}

func isLowerASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			return false
		}
	}
	return true
}

// renderAttrs returns the attributes to write, adding rel="noopener" to links
// that open a new browsing context.
func (e *Element) renderAttrs(tag string) []attr {
	i := e.index("target")
	if i < 0 || strings.ToLower(e.attrs[i].value) != "_blank" || (tag != "a" && tag != "area" && tag != "form") {
		return e.attrs
	}
	attrs := slices.Clone(e.attrs)
	if j := e.index("rel"); j >= 0 {
		rel := strings.Fields(strings.ToLower(attrs[j].value))
		if !slices.Contains(rel, "noopener") && !slices.Contains(rel, "noreferrer") {
			attrs[j].value = strings.TrimSpace(attrs[j].value + " noopener")
		}
		return attrs
	}
	return append(attrs, attr{name: "rel", value: "noopener noreferrer"})
}

// attrValue applies the protection that matches the attribute's kind.
func (e *Element) attrValue(tag string, a attr) (string, error) {
	if a.trusted {
		return a.value, nil
	}
	lname := strings.ToLower(a.name)
	switch classifyAttr(a.name) {
	case kindJS:
		return "", &SecurityError{Tag: e.tag, Attr: a.name, Reason: "attribute executes JavaScript; use On or JSAttr with an UnsafeJS value"}
	case kindHTML:
		return "", &SecurityError{Tag: e.tag, Attr: a.name, Reason: "attribute contains HTML; use UnsafeAttr"}
	case kindURL:
		if isSafeURL(a.value, lname == "src" || lname == "poster") {
			return a.value, nil
		}
		return FilteredValue, nil
	case kindSrcset:
		return sanitizeSrcset(a.value), nil
	case kindStyle:
		return sanitizeStyle(a.value), nil
	}

	switch {
	// <meta http-equiv="refresh" content="0;url=javascript:...">
	case tag == "meta" && lname == "content":
		if v, ok := e.GetAttr("http-equiv"); ok && strings.EqualFold(strings.TrimSpace(v), "refresh") {
			return sanitizeRefresh(a.value), nil
		}
	// SVG animation can rewrite href to a javascript: URL:
	// <animate attributeName="href" values="javascript:...">
	case (tag == "animate" || tag == "set") && (lname == "values" || lname == "to" || lname == "from" || lname == "by"):
		if target, ok := e.GetAttr("attributeName"); ok {
			k := classifyAttr(strings.TrimSpace(target))
			if k == kindJS || k == kindHTML {
				return "", &SecurityError{Tag: e.tag, Attr: a.name, Reason: "animation targets a script attribute"}
			}
			if k == kindURL {
				for _, v := range strings.Split(a.value, ";") {
					if !isSafeURL(v, false) {
						return FilteredValue, nil
					}
				}
			}
		}
	}
	return a.value, nil
}

func sanitizeRefresh(v string) string {
	i := strings.Index(strings.ToLower(v), "url=")
	if i < 0 {
		return v
	}
	u := strings.Trim(strings.TrimSpace(v[i+4:]), `'"`)
	if isSafeURL(u, false) {
		return v
	}
	return v[:i] + "url=" + FilteredValue
}

// renderRawText writes the content of <script> or <style>. Only the matching
// Unsafe type is accepted, and it must not contain anything that would end
// the element early.
func (e *Element) renderRawText(r *renderer, tag string, extract func(any) (string, bool)) {
	for _, c := range e.children {
		s, ok := extract(c)
		if !ok {
			want := "UnsafeJS"
			if tag == "style" {
				want = "UnsafeCSS"
			}
			r.fail(&SecurityError{Tag: e.tag, Reason: fmt.Sprintf("content must be %s, got %T", want, c)})
			return
		}
		l := strings.ToLower(s)
		if strings.Contains(l, "</"+tag) || (tag == "script" && strings.Contains(l, "<!--")) {
			r.fail(&SecurityError{Tag: e.tag, Reason: "content would terminate the element early"})
			return
		}
		r.str(s)
	}
}

// Errored returns an element that fails to render with err. Components use it
// to report misuse (such as asChild without a single element child) from
// methods that cannot return an error.
func Errored(err error) *Element { return &Element{tag: "template", err: err} }

func renderChildren(r *renderer, children []any) {
	for _, c := range children {
		if s, ok := c.(string); ok {
			r.escape(s)
			r.maybeFlush()
			continue
		}
		r.node(c.(Node))
	}
}
