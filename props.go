package libhtml

import "strconv"

// Prop is an attribute (or any other change) applied to the element it is
// passed to as a child:
//
//	h.Button(h.Type("submit"), h.Class("btn"), h.Disabled(), "Save")
//
// Custom props are plain functions: h.Prop(func(e *h.Element) { ... }).
type Prop func(e *Element)

// Props combines several props into one.
func Props(props ...Prop) Prop {
	return func(e *Element) {
		for _, p := range props {
			if p != nil {
				p(e)
			}
		}
	}
}

// Attr sets an attribute. See Element.Attr for the protections that apply.
func Attr(name, value string) Prop { return func(e *Element) { e.Attr(name, value) } }

// UnsafeAttr sets an attribute without value sanitization. See
// Element.UnsafeAttr.
func UnsafeAttr(name, value string) Prop { return func(e *Element) { e.UnsafeAttr(name, value) } }

// Bool sets a boolean attribute such as "disabled".
func Bool(name string) Prop { return func(e *Element) { e.Bool(name, true) } }

// Class appends classes.
func Class(classes ...string) Prop { return func(e *Element) { e.Class(classes...) } }

// ClassIf appends classes when cond is true.
func ClassIf(cond bool, classes ...string) Prop {
	return func(e *Element) { e.ClassIf(cond, classes...) }
}

// On sets an inline event handler. See Element.On.
func On(event string, js UnsafeJS) Prop { return func(e *Element) { e.On(event, js) } }

// JSAttr sets a framework attribute evaluated as JavaScript. See
// Element.JSAttr.
func JSAttr(name string, js UnsafeJS) Prop { return func(e *Element) { e.JSAttr(name, js) } }

// Aria sets aria-<name>.
func Aria(name, value string) Prop { return Attr("aria-"+name, value) }

// Data sets data-<name>.
func Data(name, value string) Prop { return Attr("data-"+name, value) }

// TabIndex sets tabindex.
func TabIndex(i int) Prop { return Attr("tabindex", strconv.Itoa(i)) }

// String-valued attributes.
func ID(v string) Prop           { return Attr("id", v) }
func Href(v string) Prop         { return Attr("href", v) }
func Src(v string) Prop          { return Attr("src", v) }
func Alt(v string) Prop          { return Attr("alt", v) }
func Type(v string) Prop         { return Attr("type", v) }
func Name(v string) Prop         { return Attr("name", v) }
func Value(v string) Prop        { return Attr("value", v) }
func Placeholder(v string) Prop  { return Attr("placeholder", v) }
func For(v string) Prop          { return Attr("for", v) }
func Action(v string) Prop       { return Attr("action", v) }
func Method(v string) Prop       { return Attr("method", v) }
func Target(v string) Prop       { return Attr("target", v) }
func Rel(v string) Prop          { return Attr("rel", v) }
func Role(v string) Prop         { return Attr("role", v) }
func TitleAttr(v string) Prop    { return Attr("title", v) }
func Style(v string) Prop        { return Attr("style", v) }
func Lang(v string) Prop         { return Attr("lang", v) }
func Dir(v string) Prop          { return Attr("dir", v) }
func Nonce(v string) Prop        { return Attr("nonce", v) }
func Content(v string) Prop      { return Attr("content", v) }
func Charset(v string) Prop      { return Attr("charset", v) }
func HTTPEquiv(v string) Prop    { return Attr("http-equiv", v) }
func Integrity(v string) Prop    { return Attr("integrity", v) }
func CrossOrigin(v string) Prop  { return Attr("crossorigin", v) }
func Width(v string) Prop        { return Attr("width", v) }
func Height(v string) Prop       { return Attr("height", v) }
func AutoComplete(v string) Prop { return Attr("autocomplete", v) }
func Min(v string) Prop          { return Attr("min", v) }
func Max(v string) Prop          { return Attr("max", v) }
func Step(v string) Prop         { return Attr("step", v) }
func Pattern(v string) Prop      { return Attr("pattern", v) }
func EncType(v string) Prop      { return Attr("enctype", v) }
func FormAttr(v string) Prop     { return Attr("form", v) }
func LabelAttr(v string) Prop    { return Attr("label", v) }
func Download(v string) Prop     { return Attr("download", v) }
func Loading(v string) Prop      { return Attr("loading", v) }

// SVG attributes.
func Xmlns(v string) Prop          { return Attr("xmlns", v) }
func ViewBox(v string) Prop        { return Attr("viewBox", v) }
func Fill(v string) Prop           { return Attr("fill", v) }
func Stroke(v string) Prop         { return Attr("stroke", v) }
func StrokeWidth(v string) Prop    { return Attr("stroke-width", v) }
func StrokeLinecap(v string) Prop  { return Attr("stroke-linecap", v) }
func StrokeLinejoin(v string) Prop { return Attr("stroke-linejoin", v) }
func D(v string) Prop              { return Attr("d", v) }

// Boolean attributes. Combine with If for conditional ones:
// h.If(busy, h.Disabled()).
func Disabled() Prop  { return Bool("disabled") }
func Checked() Prop   { return Bool("checked") }
func Selected() Prop  { return Bool("selected") }
func Required() Prop  { return Bool("required") }
func ReadOnly() Prop  { return Bool("readonly") }
func Hidden() Prop    { return Bool("hidden") }
func AutoFocus() Prop { return Bool("autofocus") }
func Multiple() Prop  { return Bool("multiple") }
func Open() Prop      { return Bool("open") }
func Defer() Prop     { return Bool("defer") }
func Async() Prop     { return Bool("async") }

// --- builder-method equivalents ------------------------------------------------

func (e *Element) ID(v string) *Element           { return e.Attr("id", v) }
func (e *Element) Href(v string) *Element         { return e.Attr("href", v) }
func (e *Element) Src(v string) *Element          { return e.Attr("src", v) }
func (e *Element) Alt(v string) *Element          { return e.Attr("alt", v) }
func (e *Element) Type(v string) *Element         { return e.Attr("type", v) }
func (e *Element) Name(v string) *Element         { return e.Attr("name", v) }
func (e *Element) Value(v string) *Element        { return e.Attr("value", v) }
func (e *Element) Placeholder(v string) *Element  { return e.Attr("placeholder", v) }
func (e *Element) For(v string) *Element          { return e.Attr("for", v) }
func (e *Element) Action(v string) *Element       { return e.Attr("action", v) }
func (e *Element) Method(v string) *Element       { return e.Attr("method", v) }
func (e *Element) Target(v string) *Element       { return e.Attr("target", v) }
func (e *Element) Rel(v string) *Element          { return e.Attr("rel", v) }
func (e *Element) Role(v string) *Element         { return e.Attr("role", v) }
func (e *Element) Title(v string) *Element        { return e.Attr("title", v) }
func (e *Element) Style(v string) *Element        { return e.Attr("style", v) }
func (e *Element) Lang(v string) *Element         { return e.Attr("lang", v) }
func (e *Element) Nonce(v string) *Element        { return e.Attr("nonce", v) }
func (e *Element) Content(v string) *Element      { return e.Attr("content", v) }
func (e *Element) AutoComplete(v string) *Element { return e.Attr("autocomplete", v) }
func (e *Element) Form(v string) *Element         { return e.Attr("form", v) }
func (e *Element) Aria(name, v string) *Element   { return e.Attr("aria-"+name, v) }
func (e *Element) Data(name, v string) *Element   { return e.Attr("data-"+name, v) }
func (e *Element) TabIndex(i int) *Element        { return e.Attr("tabindex", strconv.Itoa(i)) }

func (e *Element) Disabled(on bool) *Element  { return e.Bool("disabled", on) }
func (e *Element) Checked(on bool) *Element   { return e.Bool("checked", on) }
func (e *Element) Selected(on bool) *Element  { return e.Bool("selected", on) }
func (e *Element) Required(on bool) *Element  { return e.Bool("required", on) }
func (e *Element) ReadOnly(on bool) *Element  { return e.Bool("readonly", on) }
func (e *Element) Hidden(on bool) *Element    { return e.Bool("hidden", on) }
func (e *Element) AutoFocus(on bool) *Element { return e.Bool("autofocus", on) }
func (e *Element) Multiple(on bool) *Element  { return e.Bool("multiple", on) }
