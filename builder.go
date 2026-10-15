package libhtml

import (
	"fmt"
	"io"
	"strings"
)

// Builder gives a component the standard builder methods while keeping its
// own type, so component-specific and generic calls chain freely:
//
//	type ButtonBuilder struct {
//		h.Builder[ButtonBuilder]
//		variant string
//	}
//
//	func Button(children ...any) *ButtonBuilder {
//		b := &ButtonBuilder{}
//		b.Builder = h.NewBuilder(b, "button")
//		return b.Children(children...)
//	}
//
//	ui.Button("Save").Class("w-full").Variant("outline").Disabled(true)
//
// The component then implements Render (and usually Element) by finishing
// the element returned by Elem.
type Builder[T any] struct {
	self *T
	el   *Element
}

// ChildAcceptor lets a component consume some of its children itself, such
// as typed variant values. AcceptChild returns true when it handled c.
type ChildAcceptor interface {
	AcceptChild(c any) bool
}

// NewBuilder returns a Builder for self that collects attributes and children
// on an element with the given tag.
func NewBuilder[T any](self *T, tag string) Builder[T] {
	return Builder[T]{self: self, el: El(tag)}
}

// Elem returns the element holding the attributes and children set so far.
func (b *Builder[T]) Elem() *Element { return b.el }

// Children appends children (see Element.Children). If the component
// implements ChildAcceptor, it sees every child first.
func (b *Builder[T]) Children(children ...any) *T {
	acc, _ := any(b.self).(ChildAcceptor)
	for _, c := range children {
		b.add(acc, c)
	}
	return b.self
}

func (b *Builder[T]) add(acc ChildAcceptor, c any) {
	switch v := c.(type) {
	case Fragment:
		for _, x := range v {
			b.add(acc, x)
		}
		return
	case []any:
		for _, x := range v {
			b.add(acc, x)
		}
		return
	}
	if acc != nil && c != nil && acc.AcceptChild(c) {
		return
	}
	b.el.add(c)
}

func (b *Builder[T]) Attr(name, value string) *T { b.el.Attr(name, value); return b.self }
func (b *Builder[T]) AttrIf(cond bool, name, value string) *T {
	b.el.AttrIf(cond, name, value)
	return b.self
}
func (b *Builder[T]) UnsafeAttr(name, value string) *T { b.el.UnsafeAttr(name, value); return b.self }
func (b *Builder[T]) Bool(name string, on bool) *T     { b.el.Bool(name, on); return b.self }
func (b *Builder[T]) Class(classes ...string) *T       { b.el.Class(classes...); return b.self }
func (b *Builder[T]) ClassIf(cond bool, classes ...string) *T {
	b.el.ClassIf(cond, classes...)
	return b.self
}
func (b *Builder[T]) ID(v string) *T                     { b.el.ID(v); return b.self }
func (b *Builder[T]) Title(v string) *T                  { b.el.Title(v); return b.self }
func (b *Builder[T]) Style(v string) *T                  { b.el.Style(v); return b.self }
func (b *Builder[T]) Role(v string) *T                   { b.el.Role(v); return b.self }
func (b *Builder[T]) Aria(name, v string) *T             { b.el.Aria(name, v); return b.self }
func (b *Builder[T]) Data(name, v string) *T             { b.el.Data(name, v); return b.self }
func (b *Builder[T]) TabIndex(i int) *T                  { b.el.TabIndex(i); return b.self }
func (b *Builder[T]) Hidden(on bool) *T                  { b.el.Hidden(on); return b.self }
func (b *Builder[T]) On(event string, js UnsafeJS) *T    { b.el.On(event, js); return b.self }
func (b *Builder[T]) JSAttr(name string, js UnsafeJS) *T { b.el.JSAttr(name, js); return b.self }
func (b *Builder[T]) Text(s ...string) *T                { b.el.Text(s...); return b.self }
func (b *Builder[T]) Textf(format string, args ...any) *T {
	b.el.Textf(format, args...)
	return b.self
}

// With applies fn to the component.
func (b *Builder[T]) With(fn func(*T)) *T { fn(b.self); return b.self }

// When applies fn to the component only when cond is true.
func (b *Builder[T]) When(cond bool, fn func(*T)) *T {
	if cond {
		fn(b.self)
	}
	return b.self
}

// Slot merges the attributes of slot into child and returns the result,
// implementing the Radix "asChild" pattern: a component can hand its
// behavior and styling to the element the caller provides.
//
// The child's attributes win, except that classes are combined with
// mergeClass (slot classes first; nil means plain concatenation), styles are
// concatenated, and event handlers set on both run slot first, then child.
// The child keeps its own children. child must be an *Element or an
// Elementer.
func Slot(slot *Element, child any, mergeClass func(...string) string) (*Element, error) {
	var target *Element
	switch c := child.(type) {
	case *Element:
		target = c
	case Elementer:
		target = c.Element()
	}
	if target == nil {
		return nil, fmt.Errorf("libhtml: Slot needs an *Element or Elementer child, got %T", child)
	}
	if slot.err != nil {
		return nil, slot.err
	}
	out := target.Clone()
	for _, a := range slot.attrs {
		i := out.index(a.name)
		if i < 0 {
			out.attrs = append(out.attrs, a)
			continue
		}
		cur := &out.attrs[i]
		switch lname := strings.ToLower(a.name); {
		case lname == "style":
			cur.value = strings.TrimSuffix(strings.TrimSpace(a.value), ";") + ";" + cur.value
			cur.trusted = cur.trusted && a.trusted
		case len(lname) > 2 && strings.HasPrefix(lname, "on") && a.trusted && cur.trusted:
			cur.value = a.value + ";" + cur.value
		}
	}
	slotClass, childClass := slot.ClassName(), out.ClassName()
	if mergeClass != nil {
		out.SetClass(mergeClass(slotClass, childClass))
	} else {
		out.SetClass(slotClass + " " + childClass)
	}
	return out, nil
}

// Spread returns a copy of base with the attributes, classes and children of
// props applied on top: the JSX pattern <El defaultProp="x" {...props} />.
// Attributes set on props replace base's, classes are combined with
// mergeClass (base classes first; nil means plain concatenation) and props'
// children follow base's. Neither argument is modified.
func Spread(base, props *Element, mergeClass func(...string) string) *Element {
	out := base.Clone()
	if props == nil {
		return out
	}
	if out.err == nil {
		out.err = props.err
	}
	for _, a := range props.attrs {
		if i := out.index(a.name); i >= 0 {
			out.attrs[i] = a
		} else {
			out.attrs = append(out.attrs, a)
		}
	}
	if len(props.classes) > 0 {
		if mergeClass != nil {
			out.SetClass(mergeClass(out.ClassName(), props.ClassName()))
		} else {
			out.Class(props.classes...)
		}
	}
	out.children = append(out.children, props.children...)
	return out
}

// ElementerIn is implemented by components whose element depends on
// render-scoped values (see WithValue), such as the visual style set by
// style.Scope. ElementIn resolves the element as it renders inside w.
type ElementerIn interface {
	ElementIn(w io.Writer) *Element
}

// ResolveIn returns child's element as it renders inside w: ElementIn(w)
// for an ElementerIn, Element() for an Elementer, the element itself for an
// *Element, and nil otherwise.
func ResolveIn(w io.Writer, child any) *Element {
	switch c := child.(type) {
	case ElementerIn:
		return c.ElementIn(w)
	case *Element:
		return c
	case Elementer:
		return c.Element()
	}
	return nil
}

// SlotIn is Slot for use inside a Render method: the child is resolved with
// ResolveIn, so components rendered through asChild keep the render-scoped
// values (e.g. the style) of where they are rendered.
func SlotIn(w io.Writer, slot *Element, child any, mergeClass func(...string) string) (*Element, error) {
	if el := ResolveIn(w, child); el != nil {
		return Slot(slot, el, mergeClass)
	}
	return Slot(slot, child, mergeClass)
}
