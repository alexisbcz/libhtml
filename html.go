/**
 * Copyright 2025 Alexis Bouchez <alexbcz@proton.me>
 *
 * This file is part of libhtml.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package html

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// Attribute represents HTML element attributes
type Attribute map[string]string

// Node interface defines components that can render themselves
type Node interface {
	Render(w io.Writer) error
}

// document represents an HTML document with its structure
type document struct {
	children []Node
}

// Render implements Node for document, rendering a complete HTML document
func (d *document) Render(w io.Writer) (err error) {
	if _, err := w.Write([]byte("<!DOCTYPE html>")); err != nil {
		return err
	}
	for _, child := range d.children {
		if child == nil {
			continue
		}
		if err := child.Render(w); err != nil {
			return err
		}
	}
	return nil
}

// Document creates a complete HTML document with the given children
func Document(children ...Node) *document {
	return &document{
		children: children,
	}
}

// Children sets the children for a given document
func (d *document) Children(children ...Node) *document {
	d.children = children
	return d
}

// text renders escaped text content
type text struct {
	content string
}

// Text creates a node that renders HTML-escaped text content
func Text(content string) Node {
	return &text{content: content}
}

// Textf creates a node that renders formatted HTML-escaped text
func Textf(format string, args ...any) Node {
	return &text{content: fmt.Sprintf(format, args...)}
}

// Render implements Node.Render for text
func (t *text) Render(w io.Writer) error {
	_, err := io.WriteString(w, html.EscapeString(t.content))
	return err
}

// raw renders content as-is without escaping
type raw struct {
	content string
}

// Raw creates a node that renders raw HTML content
func Raw(content string) Node {
	return &raw{content: content}
}

// Rawf creates a node that renders formatted raw HTML content
func Rawf(format string, args ...any) Node {
	return &raw{content: fmt.Sprintf(format, args...)}
}

// Render implements Node.Render for raw
func (r *raw) Render(w io.Writer) error {
	_, err := io.WriteString(w, r.content)
	return err
}

// if_ conditionally renders content based on a condition
type if_ struct {
	condition bool
	then      Node
}

// If conditionally renders content when condition is true
func If(condition bool, then Node) Node {
	return &if_{
		condition: condition,
		then:      then,
	}
}

// ifFunc is a lazy conditional renderer that only evaluates its content when true
type ifFunc struct {
	condition bool
	thenFn    func() Node
}

// IfFunc conditionally renders content when condition is true
// Uses a callback function to avoid evaluating the content when condition is false
func IfFunc(condition bool, thenFn func() Node) Node {
	return &ifFunc{
		condition: condition,
		thenFn:    thenFn,
	}
}

// Render implements Node.Render for ifFunc
func (i *ifFunc) Render(w io.Writer) error {
	if i.condition && i.thenFn != nil {
		node := i.thenFn()
		if node != nil {
			return node.Render(w)
		}
	}
	return nil
}

// Render implements Node.Render for if_
func (i *if_) Render(w io.Writer) error {
	if i.condition {
		return i.then.Render(w)
	}
	return nil
}

// ifElse conditionally renders one of two contents based on a condition
type ifElse struct {
	condition bool
	then      Node
	else_     Node
}

// IfElse conditionally renders one of two nodes based on condition
func IfElse(condition bool, then Node, else_ Node) Node {
	return &ifElse{
		condition: condition,
		then:      then,
		else_:     else_,
	}
}

// Render implements Node.Render for ifElse
func (ie *ifElse) Render(w io.Writer) error {
	if ie.condition {
		return ie.then.Render(w)
	}
	return ie.else_.Render(w)
}

// ifElseFunc is a lazy conditional renderer that only evaluates its content when true
type ifElseFunc struct {
	condition bool
	thenFn    func() Node
	elseFn    func() Node
}

// IfElseFunc conditionally renders content when condition is true
// Uses a callback function to avoid evaluating the content when condition is false
func IfElseFunc(condition bool, thenFn, elseFn func() Node) Node {
	return &ifElseFunc{
		condition: condition,
		thenFn:    thenFn,
		elseFn:    elseFn,
	}
}

// Render implements Node.Render for ifElseFunc
func (i *ifElseFunc) Render(w io.Writer) error {
	if i.condition && i.thenFn != nil {
		node := i.thenFn()
		if node != nil {
			return node.Render(w)
		}
		return nil
	}
	return i.elseFn().Render(w)
}

// map_ renders a collection of items using a mapping function
type map_[T any] struct {
	items     []T
	transform func(item T) Node
}

// Map renders a collection of items using a transform function
func Map[T any](items []T, transform func(item T) Node) Node {
	return &map_[T]{
		items:     items,
		transform: transform,
	}
}

// Render implements Node.Render for map_
func (m *map_[T]) Render(w io.Writer) error {
	for _, item := range m.items {
		node := m.transform(item)
		if err := node.Render(w); err != nil {
			return err
		}
	}
	return nil
}

// group represents a collection of nodes with no root element
type group struct {
	children []Node
}

// Group combines multiple nodes without a wrapper element
func Group(children ...Node) Node {
	return &group{children: children}
}

// Render implements Node.Render for group
func (g *group) Render(w io.Writer) error {
	for _, child := range g.children {
		if child == nil {
			continue
		}
		if err := child.Render(w); err != nil {
			return err
		}
	}
	return nil
}

// Tag represents the base structure for all HTML elements
type Tag struct {
	// name of the HTML element (e.g., "div", "p", "a")
	name string

	// isVoid indicates if the element cannot have children
	isVoid bool

	// children nodes of the current HTML element
	children []Node

	// attributes stores the element's HTML attributes
	attributes map[string]string
}

// NewTag creates a new Tag instance with specified properties
func NewTag(name string, isVoid bool, children []Node) *Tag {
	return &Tag{
		name:       name,
		isVoid:     isVoid,
		children:   children,
		attributes: make(map[string]string),
	}
}

// Children set the children for a given tag.
func (e *Tag) Children(children ...Node) *Tag {
	e.children = children
	return e
}

// Render implements Node.
func (e *Tag) Render(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "<%s", e.name); err != nil {
		return err
	}

	// Render attributes
	for key, value := range e.attributes {
		if _, err := fmt.Fprintf(w, " %s=\"%s\"", key, value); err != nil {
			return err
		}
	}

	if e.isVoid {
		_, err := w.Write([]byte("/>"))
		return err
	}

	// Write closing bracket for opening tag
	if _, err := w.Write([]byte(">")); err != nil {
		return err
	}

	// Render all children
	for _, child := range e.children {
		if child == nil {
			continue
		}
		if err := child.Render(w); err != nil {
			return err
		}
	}

	// Write closing tag
	_, err := fmt.Fprintf(w, "</%s>", e.name)
	return err
}

// Attribute adds or updates an attribute for the tag
// Allows method chaining for fluent interface
func (t *Tag) Attribute(key, value string) *Tag {
	if value == "" {
		return t
	}
	t.attributes[key] = value
	return t
}

// Attribute adds or updates an attribute for the tag
// Allows method chaining for fluent interface
func (t *Tag) AttributeIf(cond bool, key, value string) *Tag {
	if cond {
		t.attributes[key] = value
	}
	return t
}

// A represents the <a> HTML element
type a struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// A creates a new a element
// Allows optional child nodes to be passed during creation
func A(children ...Node) *a {
	return &a{NewTag("a", false, children)}
}

// Style sets the "style" attribute
// Returns the element itself to enable method chaining
func (e *a) Style(value string) *a {
	e.Attribute("style", value)
	return e
}

// StyleIf conditionally sets the "style" attribute
// Only sets the attribute if the condition is true
func (e *a) StyleIf(condition bool, value string) *a {
	if condition {
		e.Attribute("style", value)
	}
	return e
}

// ID sets the "id" attribute
// Returns the element itself to enable method chaining
func (e *a) ID(value string) *a {
	e.Attribute("id", value)
	return e
}

// IDIf conditionally sets the "id" attribute
// Only sets the attribute if the condition is true
func (e *a) IDIf(condition bool, value string) *a {
	if condition {
		e.Attribute("id", value)
	}
	return e
}

// Class sets the "class" attribute
// Returns the element itself to enable method chaining
func (e *Tag) Class(values ...string) *Tag {
	e.Attribute("class", strings.Join(values, " "))
	return e
}

// ClassIf conditionally sets the "class" attribute
// Only sets the attribute if the condition is true
func (e *Tag) ClassIf(condition bool, value string) *Tag {
	if condition {
		e.Attribute("class", value)
	}
	return e
}

// Href sets the "href" attribute
// Returns the element itself to enable method chaining
func (e *a) Href(value string) *a {
	e.Attribute("href", value)
	return e
}

// HrefIf conditionally sets the "href" attribute
// Only sets the attribute if the condition is true
func (e *a) HrefIf(condition bool, value string) *a {
	if condition {
		e.Attribute("href", value)
	}
	return e
}

// Target sets the "target" attribute
// Returns the element itself to enable method chaining
func (e *a) Target(value string) *a {
	e.Attribute("target", value)
	return e
}

// TargetIf conditionally sets the "target" attribute
// Only sets the attribute if the condition is true
func (e *a) TargetIf(condition bool, value string) *a {
	if condition {
		e.Attribute("target", value)
	}
	return e
}

// Download sets the "download" attribute
// Returns the element itself to enable method chaining
func (e *a) Download(value string) *a {
	e.Attribute("download", value)
	return e
}

// DownloadIf conditionally sets the "download" attribute
// Only sets the attribute if the condition is true
func (e *a) DownloadIf(condition bool, value string) *a {
	if condition {
		e.Attribute("download", value)
	}
	return e
}

// Rel sets the "rel" attribute
// Returns the element itself to enable method chaining
func (e *a) Rel(value string) *a {
	e.Attribute("rel", value)
	return e
}

// RelIf conditionally sets the "rel" attribute
// Only sets the attribute if the condition is true
func (e *a) RelIf(condition bool, value string) *a {
	if condition {
		e.Attribute("rel", value)
	}
	return e
}

// Type sets the "type" attribute
// Returns the element itself to enable method chaining
func (e *a) Type(value string) *a {
	e.Attribute("type", value)
	return e
}

// TypeIf conditionally sets the "type" attribute
// Only sets the attribute if the condition is true
func (e *a) TypeIf(condition bool, value string) *a {
	if condition {
		e.Attribute("type", value)
	}
	return e
}

// Hreflang sets the "hreflang" attribute
// Returns the element itself to enable method chaining
func (e *a) Hreflang(value string) *a {
	e.Attribute("hreflang", value)
	return e
}

// HreflangIf conditionally sets the "hreflang" attribute
// Only sets the attribute if the condition is true
func (e *a) HreflangIf(condition bool, value string) *a {
	if condition {
		e.Attribute("hreflang", value)
	}
	return e
}

// Media sets the "media" attribute
// Returns the element itself to enable method chaining
func (e *a) Media(value string) *a {
	e.Attribute("media", value)
	return e
}

// MediaIf conditionally sets the "media" attribute
// Only sets the attribute if the condition is true
func (e *a) MediaIf(condition bool, value string) *a {
	if condition {
		e.Attribute("media", value)
	}
	return e
}

// Ping sets the "ping" attribute
// Returns the element itself to enable method chaining
func (e *a) Ping(value string) *a {
	e.Attribute("ping", value)
	return e
}

// PingIf conditionally sets the "ping" attribute
// Only sets the attribute if the condition is true
func (e *a) PingIf(condition bool, value string) *a {
	if condition {
		e.Attribute("ping", value)
	}
	return e
}

// Referrerpolicy sets the "referrerpolicy" attribute
// Returns the element itself to enable method chaining
func (e *a) Referrerpolicy(value string) *a {
	e.Attribute("referrerpolicy", value)
	return e
}

// ReferrerpolicyIf conditionally sets the "referrerpolicy" attribute
// Only sets the attribute if the condition is true
func (e *a) ReferrerpolicyIf(condition bool, value string) *a {
	if condition {
		e.Attribute("referrerpolicy", value)
	}
	return e
}

// Accesskey sets the "accesskey" attribute
// Returns the element itself to enable method chaining
func (e *a) Accesskey(value string) *a {
	e.Attribute("accesskey", value)
	return e
}

// AccesskeyIf conditionally sets the "accesskey" attribute
// Only sets the attribute if the condition is true
func (e *a) AccesskeyIf(condition bool, value string) *a {
	if condition {
		e.Attribute("accesskey", value)
	}
	return e
}

// Contenteditable sets the "contenteditable" attribute
// Returns the element itself to enable method chaining
func (e *a) Contenteditable(value string) *a {
	e.Attribute("contenteditable", value)
	return e
}

// ContenteditableIf conditionally sets the "contenteditable" attribute
// Only sets the attribute if the condition is true
func (e *a) ContenteditableIf(condition bool, value string) *a {
	if condition {
		e.Attribute("contenteditable", value)
	}
	return e
}

// Dir sets the "dir" attribute
// Returns the element itself to enable method chaining
func (e *a) Dir(value string) *a {
	e.Attribute("dir", value)
	return e
}

// DirIf conditionally sets the "dir" attribute
// Only sets the attribute if the condition is true
func (e *a) DirIf(condition bool, value string) *a {
	if condition {
		e.Attribute("dir", value)
	}
	return e
}

// Draggable sets the "draggable" attribute
// Returns the element itself to enable method chaining
func (e *a) Draggable(value string) *a {
	e.Attribute("draggable", value)
	return e
}

// DraggableIf conditionally sets the "draggable" attribute
// Only sets the attribute if the condition is true
func (e *a) DraggableIf(condition bool, value string) *a {
	if condition {
		e.Attribute("draggable", value)
	}
	return e
}

// Hidden sets the "hidden" attribute
// Returns the element itself to enable method chaining
func (e *a) Hidden(value string) *a {
	e.Attribute("hidden", value)
	return e
}

// HiddenIf conditionally sets the "hidden" attribute
// Only sets the attribute if the condition is true
func (e *a) HiddenIf(condition bool, value string) *a {
	if condition {
		e.Attribute("hidden", value)
	}
	return e
}

// Lang sets the "lang" attribute
// Returns the element itself to enable method chaining
func (e *a) Lang(value string) *a {
	e.Attribute("lang", value)
	return e
}

// LangIf conditionally sets the "lang" attribute
// Only sets the attribute if the condition is true
func (e *a) LangIf(condition bool, value string) *a {
	if condition {
		e.Attribute("lang", value)
	}
	return e
}

// Spellcheck sets the "spellcheck" attribute
// Returns the element itself to enable method chaining
func (e *a) Spellcheck(value string) *a {
	e.Attribute("spellcheck", value)
	return e
}

// SpellcheckIf conditionally sets the "spellcheck" attribute
// Only sets the attribute if the condition is true
func (e *a) SpellcheckIf(condition bool, value string) *a {
	if condition {
		e.Attribute("spellcheck", value)
	}
	return e
}

// Tabindex sets the "tabindex" attribute
// Returns the element itself to enable method chaining
func (e *a) Tabindex(value string) *a {
	e.Attribute("tabindex", value)
	return e
}

// TabindexIf conditionally sets the "tabindex" attribute
// Only sets the attribute if the condition is true
func (e *a) TabindexIf(condition bool, value string) *a {
	if condition {
		e.Attribute("tabindex", value)
	}
	return e
}

// Title sets the "title" attribute
// Returns the element itself to enable method chaining
func (e *a) Title(value string) *a {
	e.Attribute("title", value)
	return e
}

// TitleIf conditionally sets the "title" attribute
// Only sets the attribute if the condition is true
func (e *a) TitleIf(condition bool, value string) *a {
	if condition {
		e.Attribute("title", value)
	}
	return e
}

// Translate sets the "translate" attribute
// Returns the element itself to enable method chaining
func (e *a) Translate(value string) *a {
	e.Attribute("translate", value)
	return e
}

// TranslateIf conditionally sets the "translate" attribute
// Only sets the attribute if the condition is true
func (e *a) TranslateIf(condition bool, value string) *a {
	if condition {
		e.Attribute("translate", value)
	}
	return e
}

// Role sets the "role" attribute
// Returns the element itself to enable method chaining
func (e *a) Role(value string) *a {
	e.Attribute("role", value)
	return e
}

// RoleIf conditionally sets the "role" attribute
// Only sets the attribute if the condition is true
func (e *a) RoleIf(condition bool, value string) *a {
	if condition {
		e.Attribute("role", value)
	}
	return e
}

// Abbr represents the <abbr> HTML element
type abbr struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Abbr creates a new abbr element
// Allows optional child nodes to be passed during creation
func Abbr(children ...Node) *abbr {
	return &abbr{NewTag("abbr", false, children)}
}

// Address represents the <address> HTML element
type address struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Address creates a new address element
// Allows optional child nodes to be passed during creation
func Address(children ...Node) *address {
	return &address{NewTag("address", false, children)}
}

// Area represents the <area> HTML element
type area struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Area creates a new area element
// Allows optional child nodes to be passed during creation
func Area(children ...Node) *area {
	return &area{NewTag("area", true, children)}
}

// Alt sets the "alt" attribute
// Returns the element itself to enable method chaining
func (e *area) Alt(value string) *area {
	e.Attribute("alt", value)
	return e
}

// AltIf conditionally sets the "alt" attribute
// Only sets the attribute if the condition is true
func (e *area) AltIf(condition bool, value string) *area {
	if condition {
		e.Attribute("alt", value)
	}
	return e
}

// Coords sets the "coords" attribute
// Returns the element itself to enable method chaining
func (e *area) Coords(value string) *area {
	e.Attribute("coords", value)
	return e
}

// CoordsIf conditionally sets the "coords" attribute
// Only sets the attribute if the condition is true
func (e *area) CoordsIf(condition bool, value string) *area {
	if condition {
		e.Attribute("coords", value)
	}
	return e
}

// Shape sets the "shape" attribute
// Returns the element itself to enable method chaining
func (e *area) Shape(value string) *area {
	e.Attribute("shape", value)
	return e
}

// ShapeIf conditionally sets the "shape" attribute
// Only sets the attribute if the condition is true
func (e *area) ShapeIf(condition bool, value string) *area {
	if condition {
		e.Attribute("shape", value)
	}
	return e
}

// Article represents the <article> HTML element
type article struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Article creates a new article element
// Allows optional child nodes to be passed during creation
func Article(children ...Node) *article {
	return &article{NewTag("article", false, children)}
}

// Aside represents the <aside> HTML element
type aside struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Aside creates a new aside element
// Allows optional child nodes to be passed during creation
func Aside(children ...Node) *aside {
	return &aside{NewTag("aside", false, children)}
}

// Audio represents the <audio> HTML element
type audio struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Audio creates a new audio element
// Allows optional child nodes to be passed during creation
func Audio(children ...Node) *audio {
	return &audio{NewTag("audio", false, children)}
}

// Src sets the "src" attribute
// Returns the element itself to enable method chaining
func (e *audio) Src(value string) *audio {
	e.Attribute("src", value)
	return e
}

// SrcIf conditionally sets the "src" attribute
// Only sets the attribute if the condition is true
func (e *audio) SrcIf(condition bool, value string) *audio {
	if condition {
		e.Attribute("src", value)
	}
	return e
}

// Controls sets the "controls" attribute
// Returns the element itself to enable method chaining
func (e *audio) Controls(value string) *audio {
	e.Attribute("controls", value)
	return e
}

// ControlsIf conditionally sets the "controls" attribute
// Only sets the attribute if the condition is true
func (e *audio) ControlsIf(condition bool, value string) *audio {
	if condition {
		e.Attribute("controls", value)
	}
	return e
}

// Autoplay sets the "autoplay" attribute
// Returns the element itself to enable method chaining
func (e *audio) Autoplay(value string) *audio {
	e.Attribute("autoplay", value)
	return e
}

// AutoplayIf conditionally sets the "autoplay" attribute
// Only sets the attribute if the condition is true
func (e *audio) AutoplayIf(condition bool, value string) *audio {
	if condition {
		e.Attribute("autoplay", value)
	}
	return e
}

// Loop sets the "loop" attribute
// Returns the element itself to enable method chaining
func (e *audio) Loop(value string) *audio {
	e.Attribute("loop", value)
	return e
}

// LoopIf conditionally sets the "loop" attribute
// Only sets the attribute if the condition is true
func (e *audio) LoopIf(condition bool, value string) *audio {
	if condition {
		e.Attribute("loop", value)
	}
	return e
}

// Muted sets the "muted" attribute
// Returns the element itself to enable method chaining
func (e *audio) Muted(value string) *audio {
	e.Attribute("muted", value)
	return e
}

// MutedIf conditionally sets the "muted" attribute
// Only sets the attribute if the condition is true
func (e *audio) MutedIf(condition bool, value string) *audio {
	if condition {
		e.Attribute("muted", value)
	}
	return e
}

// Preload sets the "preload" attribute
// Returns the element itself to enable method chaining
func (e *audio) Preload(value string) *audio {
	e.Attribute("preload", value)
	return e
}

// PreloadIf conditionally sets the "preload" attribute
// Only sets the attribute if the condition is true
func (e *audio) PreloadIf(condition bool, value string) *audio {
	if condition {
		e.Attribute("preload", value)
	}
	return e
}

// B represents the <b> HTML element
type b struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// B creates a new b element
// Allows optional child nodes to be passed during creation
func B(children ...Node) *b {
	return &b{NewTag("b", false, children)}
}

// Base represents the <base> HTML element
type base struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Base creates a new base element
// Allows optional child nodes to be passed during creation
func Base(children ...Node) *base {
	return &base{NewTag("base", true, children)}
}

// Bdi represents the <bdi> HTML element
type bdi struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Bdi creates a new bdi element
// Allows optional child nodes to be passed during creation
func Bdi(children ...Node) *bdi {
	return &bdi{NewTag("bdi", false, children)}
}

// Bdo represents the <bdo> HTML element
type bdo struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Bdo creates a new bdo element
// Allows optional child nodes to be passed during creation
func Bdo(children ...Node) *bdo {
	return &bdo{NewTag("bdo", false, children)}
}

// Blockquote represents the <blockquote> HTML element
type blockquote struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Blockquote creates a new blockquote element
// Allows optional child nodes to be passed during creation
func Blockquote(children ...Node) *blockquote {
	return &blockquote{NewTag("blockquote", false, children)}
}

// Cite sets the "cite" attribute
// Returns the element itself to enable method chaining
func (e *blockquote) Cite(value string) *blockquote {
	e.Attribute("cite", value)
	return e
}

// CiteIf conditionally sets the "cite" attribute
// Only sets the attribute if the condition is true
func (e *blockquote) CiteIf(condition bool, value string) *blockquote {
	if condition {
		e.Attribute("cite", value)
	}
	return e
}

// Body represents the <body> HTML element
type body struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Body creates a new body element
// Allows optional child nodes to be passed during creation
func Body(children ...Node) *body {
	return &body{NewTag("body", false, children)}
}

// ClassIf conditionally sets the "name" attribute
// Only sets the attribute if the condition is true
func (e *button) ClassIf(condition bool, value string) *button {
	if condition {
		e.Attribute("class", value)
	}
	return e
}

// Br represents the <br> HTML element
type br struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Br creates a new br element
// Allows optional child nodes to be passed during creation
func Br(children ...Node) *br {
	return &br{NewTag("br", true, children)}
}

// Button represents the <button> HTML element
type button struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Button creates a new button element
// Allows optional child nodes to be passed during creation
func Button(children ...Node) *button {
	return &button{NewTag("button", false, children)}
}

// Value sets the "value" attribute
// Returns the element itself to enable method chaining
func (e *button) Value(value string) *button {
	e.Attribute("value", value)
	return e
}

// ValueIf conditionally sets the "value" attribute
// Only sets the attribute if the condition is true
func (e *button) ValueIf(condition bool, value string) *button {
	if condition {
		e.Attribute("value", value)
	}
	return e
}

// Type sets the "type" attribute
// Returns the element itself to enable method chaining
func (e *button) Type(value string) *button {
	e.Attribute("type", value)
	return e
}

// TypeIf conditionally sets the "type" attribute
// Only sets the attribute if the condition is true
func (e *button) TypeIf(condition bool, value string) *button {
	if condition {
		e.Attribute("type", value)
	}
	return e
}

// Name sets the "name" attribute
// Returns the element itself to enable method chaining
func (e *button) Name(value string) *button {
	e.Attribute("name", value)
	return e
}

// NameIf conditionally sets the "name" attribute
// Only sets the attribute if the condition is true
func (e *button) NameIf(condition bool, value string) *button {
	if condition {
		e.Attribute("name", value)
	}
	return e
}

// Disabled sets the "disabled" attribute
// Returns the element itself to enable method chaining
func (e *button) Disabled(value string) *button {
	e.Attribute("disabled", value)
	return e
}

// DisabledIf conditionally sets the "disabled" attribute
// Only sets the attribute if the condition is true
func (e *button) DisabledIf(condition bool, value string) *button {
	if condition {
		e.Attribute("disabled", value)
	}
	return e
}

// Form sets the "form" attribute
// Returns the element itself to enable method chaining
func (e *button) Form(value string) *button {
	e.Attribute("form", value)
	return e
}

// FormIf conditionally sets the "form" attribute
// Only sets the attribute if the condition is true
func (e *button) FormIf(condition bool, value string) *button {
	if condition {
		e.Attribute("form", value)
	}
	return e
}

// Formaction sets the "formaction" attribute
// Returns the element itself to enable method chaining
func (e *button) Formaction(value string) *button {
	e.Attribute("formaction", value)
	return e
}

// FormactionIf conditionally sets the "formaction" attribute
// Only sets the attribute if the condition is true
func (e *button) FormactionIf(condition bool, value string) *button {
	if condition {
		e.Attribute("formaction", value)
	}
	return e
}

// Formmethod sets the "formmethod" attribute
// Returns the element itself to enable method chaining
func (e *button) Formmethod(value string) *button {
	e.Attribute("formmethod", value)
	return e
}

// FormmethodIf conditionally sets the "formmethod" attribute
// Only sets the attribute if the condition is true
func (e *button) FormmethodIf(condition bool, value string) *button {
	if condition {
		e.Attribute("formmethod", value)
	}
	return e
}

// Formenctype sets the "formenctype" attribute
// Returns the element itself to enable method chaining
func (e *button) Formenctype(value string) *button {
	e.Attribute("formenctype", value)
	return e
}

// FormenctypeIf conditionally sets the "formenctype" attribute
// Only sets the attribute if the condition is true
func (e *button) FormenctypeIf(condition bool, value string) *button {
	if condition {
		e.Attribute("formenctype", value)
	}
	return e
}

// Formtarget sets the "formtarget" attribute
// Returns the element itself to enable method chaining
func (e *button) Formtarget(value string) *button {
	e.Attribute("formtarget", value)
	return e
}

// FormtargetIf conditionally sets the "formtarget" attribute
// Only sets the attribute if the condition is true
func (e *button) FormtargetIf(condition bool, value string) *button {
	if condition {
		e.Attribute("formtarget", value)
	}
	return e
}

// Formnovalidate sets the "formnovalidate" attribute
// Returns the element itself to enable method chaining
func (e *button) Formnovalidate(value string) *button {
	e.Attribute("formnovalidate", value)
	return e
}

// FormnovalidateIf conditionally sets the "formnovalidate" attribute
// Only sets the attribute if the condition is true
func (e *button) FormnovalidateIf(condition bool, value string) *button {
	if condition {
		e.Attribute("formnovalidate", value)
	}
	return e
}

// Autofocus sets the "autofocus" attribute
// Returns the element itself to enable method chaining
func (e *button) Autofocus(value string) *button {
	e.Attribute("autofocus", value)
	return e
}

// AutofocusIf conditionally sets the "autofocus" attribute
// Only sets the attribute if the condition is true
func (e *button) AutofocusIf(condition bool, value string) *button {
	if condition {
		e.Attribute("autofocus", value)
	}
	return e
}

// Canvas represents the <canvas> HTML element
type canvas struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Canvas creates a new canvas element
// Allows optional child nodes to be passed during creation
func Canvas(children ...Node) *canvas {
	return &canvas{NewTag("canvas", false, children)}
}

// Width sets the "width" attribute
// Returns the element itself to enable method chaining
func (e *canvas) Width(value string) *canvas {
	e.Attribute("width", value)
	return e
}

// WidthIf conditionally sets the "width" attribute
// Only sets the attribute if the condition is true
func (e *canvas) WidthIf(condition bool, value string) *canvas {
	if condition {
		e.Attribute("width", value)
	}
	return e
}

// Height sets the "height" attribute
// Returns the element itself to enable method chaining
func (e *canvas) Height(value string) *canvas {
	e.Attribute("height", value)
	return e
}

// HeightIf conditionally sets the "height" attribute
// Only sets the attribute if the condition is true
func (e *canvas) HeightIf(condition bool, value string) *canvas {
	if condition {
		e.Attribute("height", value)
	}
	return e
}

// Caption represents the <caption> HTML element
type caption struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Caption creates a new caption element
// Allows optional child nodes to be passed during creation
func Caption(children ...Node) *caption {
	return &caption{NewTag("caption", false, children)}
}

// Cite represents the <cite> HTML element
type cite struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Cite creates a new cite element
// Allows optional child nodes to be passed during creation
func Cite(children ...Node) *cite {
	return &cite{NewTag("cite", false, children)}
}

// Code represents the <code> HTML element
type code struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Code creates a new code element
// Allows optional child nodes to be passed during creation
func Code(children ...Node) *code {
	return &code{NewTag("code", false, children)}
}

// Col represents the <col> HTML element
type col struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Col creates a new col element
// Allows optional child nodes to be passed during creation
func Col(children ...Node) *col {
	return &col{NewTag("col", true, children)}
}

// Span sets the "span" attribute
// Returns the element itself to enable method chaining
func (e *col) Span(value string) *col {
	e.Attribute("span", value)
	return e
}

// SpanIf conditionally sets the "span" attribute
// Only sets the attribute if the condition is true
func (e *col) SpanIf(condition bool, value string) *col {
	if condition {
		e.Attribute("span", value)
	}
	return e
}

// Colgroup represents the <colgroup> HTML element
type colgroup struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Colgroup creates a new colgroup element
// Allows optional child nodes to be passed during creation
func Colgroup(children ...Node) *colgroup {
	return &colgroup{NewTag("colgroup", false, children)}
}

// Data represents the <data> HTML element
type data struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Data creates a new data element
// Allows optional child nodes to be passed during creation
func Data(children ...Node) *data {
	return &data{NewTag("data", false, children)}
}

// Datalist represents the <datalist> HTML element
type datalist struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Datalist creates a new datalist element
// Allows optional child nodes to be passed during creation
func Datalist(children ...Node) *datalist {
	return &datalist{NewTag("datalist", false, children)}
}

// Dd represents the <dd> HTML element
type dd struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Dd creates a new dd element
// Allows optional child nodes to be passed during creation
func Dd(children ...Node) *dd {
	return &dd{NewTag("dd", false, children)}
}

// Del represents the <del> HTML element
type del struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Del creates a new del element
// Allows optional child nodes to be passed during creation
func Del(children ...Node) *del {
	return &del{NewTag("del", false, children)}
}

// Details represents the <details> HTML element
type details struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Details creates a new details element
// Allows optional child nodes to be passed during creation
func Details(children ...Node) *details {
	return &details{NewTag("details", false, children)}
}

// Open sets the "open" attribute
// Returns the element itself to enable method chaining
func (e *details) Open(value string) *details {
	e.Attribute("open", value)
	return e
}

// OpenIf conditionally sets the "open" attribute
// Only sets the attribute if the condition is true
func (e *details) OpenIf(condition bool, value string) *details {
	if condition {
		e.Attribute("open", value)
	}
	return e
}

// Dfn represents the <dfn> HTML element
type dfn struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Dfn creates a new dfn element
// Allows optional child nodes to be passed during creation
func Dfn(children ...Node) *dfn {
	return &dfn{NewTag("dfn", false, children)}
}

// Dialog represents the <dialog> HTML element
type dialog struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Dialog creates a new dialog element
// Allows optional child nodes to be passed during creation
func Dialog(children ...Node) *dialog {
	return &dialog{NewTag("dialog", false, children)}
}

// Div represents the <div> HTML element
type div struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Div creates a new div element
// Allows optional child nodes to be passed during creation
func Div(children ...Node) *div {
	return &div{NewTag("div", false, children)}
}

// Dl represents the <dl> HTML element
type dl struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Dl creates a new dl element
// Allows optional child nodes to be passed during creation
func Dl(children ...Node) *dl {
	return &dl{NewTag("dl", false, children)}
}

// Dt represents the <dt> HTML element
type dt struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Dt creates a new dt element
// Allows optional child nodes to be passed during creation
func Dt(children ...Node) *dt {
	return &dt{NewTag("dt", false, children)}
}

// Em represents the <em> HTML element
type em struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Em creates a new em element
// Allows optional child nodes to be passed during creation
func Em(children ...Node) *em {
	return &em{NewTag("em", false, children)}
}

// Embed represents the <embed> HTML element
type embed struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Embed creates a new embed element
// Allows optional child nodes to be passed during creation
func Embed(children ...Node) *embed {
	return &embed{NewTag("embed", true, children)}
}

// Fieldset represents the <fieldset> HTML element
type fieldset struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Fieldset creates a new fieldset element
// Allows optional child nodes to be passed during creation
func Fieldset(children ...Node) *fieldset {
	return &fieldset{NewTag("fieldset", false, children)}
}

// Figcaption represents the <figcaption> HTML element
type figcaption struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Figcaption creates a new figcaption element
// Allows optional child nodes to be passed during creation
func Figcaption(children ...Node) *figcaption {
	return &figcaption{NewTag("figcaption", false, children)}
}

// Figure represents the <figure> HTML element
type figure struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Figure creates a new figure element
// Allows optional child nodes to be passed during creation
func Figure(children ...Node) *figure {
	return &figure{NewTag("figure", false, children)}
}

// Footer represents the <footer> HTML element
type footer struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Footer creates a new footer element
// Allows optional child nodes to be passed during creation
func Footer(children ...Node) *footer {
	return &footer{NewTag("footer", false, children)}
}

// Form represents the <form> HTML element
type form struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Form creates a new form element
// Allows optional child nodes to be passed during creation
func Form(children ...Node) *form {
	return &form{NewTag("form", false, children)}
}

// Id sets the "id" attribute
// Returns the element itself to enable method chaining
func (e *form) Id(value string) *form {
	e.Attribute("id", value)
	return e
}

// IdIf conditionally sets the "id" attribute
// Only sets the attribute if the condition is true
func (e *form) IdIf(condition bool, value string) *form {
	if condition {
		e.Attribute("id", value)
	}
	return e
}

// Action sets the "action" attribute
// Returns the element itself to enable method chaining
func (e *form) Action(value string) *form {
	e.Attribute("action", value)
	return e
}

// ActionIf conditionally sets the "action" attribute
// Only sets the attribute if the condition is true
func (e *form) ActionIf(condition bool, value string) *form {
	if condition {
		e.Attribute("action", value)
	}
	return e
}

// Method sets the "method" attribute
// Returns the element itself to enable method chaining
func (e *form) Method(value string) *form {
	e.Attribute("method", value)
	return e
}

// MethodIf conditionally sets the "method" attribute
// Only sets the attribute if the condition is true
func (e *form) MethodIf(condition bool, value string) *form {
	if condition {
		e.Attribute("method", value)
	}
	return e
}

// Enctype sets the "enctype" attribute
// Returns the element itself to enable method chaining
func (e *form) Enctype(value string) *form {
	e.Attribute("enctype", value)
	return e
}

// EnctypeIf conditionally sets the "enctype" attribute
// Only sets the attribute if the condition is true
func (e *form) EnctypeIf(condition bool, value string) *form {
	if condition {
		e.Attribute("enctype", value)
	}
	return e
}

// Novalidate sets the "novalidate" attribute
// Returns the element itself to enable method chaining
func (e *form) Novalidate(value string) *form {
	e.Attribute("novalidate", value)
	return e
}

// NovalidateIf conditionally sets the "novalidate" attribute
// Only sets the attribute if the condition is true
func (e *form) NovalidateIf(condition bool, value string) *form {
	if condition {
		e.Attribute("novalidate", value)
	}
	return e
}

// Autocomplete sets the "autocomplete" attribute
// Returns the element itself to enable method chaining
func (e *form) Autocomplete(value string) *form {
	e.Attribute("autocomplete", value)
	return e
}

// AutocompleteIf conditionally sets the "autocomplete" attribute
// Only sets the attribute if the condition is true
func (e *form) AutocompleteIf(condition bool, value string) *form {
	if condition {
		e.Attribute("autocomplete", value)
	}
	return e
}

// H1 represents the <h1> HTML element
type h1 struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// H1 creates a new h1 element
// Allows optional child nodes to be passed during creation
func H1(children ...Node) *h1 {
	return &h1{NewTag("h1", false, children)}
}

// H2 represents the <h2> HTML element
type h2 struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// H2 creates a new h2 element
// Allows optional child nodes to be passed during creation
func H2(children ...Node) *h2 {
	return &h2{NewTag("h2", false, children)}
}

// H3 represents the <h3> HTML element
type h3 struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// H3 creates a new h3 element
// Allows optional child nodes to be passed during creation
func H3(children ...Node) *h3 {
	return &h3{NewTag("h3", false, children)}
}

// H4 represents the <h4> HTML element
type h4 struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// H4 creates a new h4 element
// Allows optional child nodes to be passed during creation
func H4(children ...Node) *h4 {
	return &h4{NewTag("h4", false, children)}
}

// H5 represents the <h5> HTML element
type h5 struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// H5 creates a new h5 element
// Allows optional child nodes to be passed during creation
func H5(children ...Node) *h5 {
	return &h5{NewTag("h5", false, children)}
}

// H6 represents the <h6> HTML element
type h6 struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// H6 creates a new h6 element
// Allows optional child nodes to be passed during creation
func H6(children ...Node) *h6 {
	return &h6{NewTag("h6", false, children)}
}

// Head represents the <head> HTML element
type head struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Head creates a new head element
// Allows optional child nodes to be passed during creation
func Head(children ...Node) *head {
	return &head{NewTag("head", false, children)}
}

// Header represents the <header> HTML element
type header struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Header creates a new header element
// Allows optional child nodes to be passed during creation
func Header(children ...Node) *header {
	return &header{NewTag("header", false, children)}
}

// Hr represents the <hr> HTML element
type hr struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Hr creates a new hr element
// Allows optional child nodes to be passed during creation
func Hr(children ...Node) *hr {
	return &hr{NewTag("hr", true, children)}
}

// Html represents the <html> HTML element
type html_ struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Lang sets the "lang" attribute
// Returns the element itself to enable method chaining
func (e *html_) Lang(value string) *html_ {
	e.Attribute("lang", value)
	return e
}

// LangIf conditionally sets the "lang" attribute
// Only sets the attribute if the condition is true
func (e *html_) LangIf(condition bool, value string) *html_ {
	if condition {
		e.Attribute("lang", value)
	}
	return e
}

// Class sets the "class" attribute
// Returns the element itself to enable method chaining
func (e *html_) Classes(values ...string) *html_ {
	e.Attribute("class", strings.Join(values, " "))
	return e
}

// ClassIf conditionally sets the "class" attribute
// Only sets the attribute if the condition is true
func (e *html_) ClassIf(condition bool, value string) *html_ {
	if condition {
		e.Attribute("class", value)
	}
	return e
}

// HTML creates a new html element
// Allows optional child nodes to be passed during creation
func HTML(children ...Node) *html_ {
	return &html_{NewTag("html", false, children)}
}

// I represents the <i> HTML element
type i struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// I creates a new i element
// Allows optional child nodes to be passed during creation
func I(children ...Node) *i {
	return &i{NewTag("i", false, children)}
}

// Iframe represents the <iframe> HTML element
type iframe struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Iframe creates a new iframe element
// Allows optional child nodes to be passed during creation
func Iframe(children ...Node) *iframe {
	return &iframe{NewTag("iframe", false, children)}
}

// Allow sets the "allow" attribute
// Returns the element itself to enable method chaining
func (e *iframe) Allow(value string) *iframe {
	e.Attribute("allow", value)
	return e
}

// AllowIf conditionally sets the "allow" attribute
// Only sets the attribute if the condition is true
func (e *iframe) AllowIf(condition bool, value string) *iframe {
	if condition {
		e.Attribute("allow", value)
	}
	return e
}

// Allowfullscreen sets the "allowfullscreen" attribute
// Returns the element itself to enable method chaining
func (e *iframe) Allowfullscreen(value string) *iframe {
	e.Attribute("allowfullscreen", value)
	return e
}

// AllowfullscreenIf conditionally sets the "allowfullscreen" attribute
// Only sets the attribute if the condition is true
func (e *iframe) AllowfullscreenIf(condition bool, value string) *iframe {
	if condition {
		e.Attribute("allowfullscreen", value)
	}
	return e
}

// Sandbox sets the "sandbox" attribute
// Returns the element itself to enable method chaining
func (e *iframe) Sandbox(value string) *iframe {
	e.Attribute("sandbox", value)
	return e
}

// SandboxIf conditionally sets the "sandbox" attribute
// Only sets the attribute if the condition is true
func (e *iframe) SandboxIf(condition bool, value string) *iframe {
	if condition {
		e.Attribute("sandbox", value)
	}
	return e
}

// Srcdoc sets the "srcdoc" attribute
// Returns the element itself to enable method chaining
func (e *iframe) Srcdoc(value string) *iframe {
	e.Attribute("srcdoc", value)
	return e
}

// SrcdocIf conditionally sets the "srcdoc" attribute
// Only sets the attribute if the condition is true
func (e *iframe) SrcdocIf(condition bool, value string) *iframe {
	if condition {
		e.Attribute("srcdoc", value)
	}
	return e
}

// Loading sets the "loading" attribute
// Returns the element itself to enable method chaining
func (e *iframe) Loading(value string) *iframe {
	e.Attribute("loading", value)
	return e
}

// LoadingIf conditionally sets the "loading" attribute
// Only sets the attribute if the condition is true
func (e *iframe) LoadingIf(condition bool, value string) *iframe {
	if condition {
		e.Attribute("loading", value)
	}
	return e
}

// Img represents the <img> HTML element
type img struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Img creates a new img element
// Allows optional child nodes to be passed during creation
func Img(children ...Node) *img {
	return &img{NewTag("img", true, children)}
}

// Srcset sets the "srcset" attribute
// Returns the element itself to enable method chaining
func (e *img) Srcset(value string) *img {
	e.Attribute("srcset", value)
	return e
}

// SrcsetIf conditionally sets the "srcset" attribute
// Only sets the attribute if the condition is true
func (e *img) SrcsetIf(condition bool, value string) *img {
	if condition {
		e.Attribute("srcset", value)
	}
	return e
}

// Sizes sets the "sizes" attribute
// Returns the element itself to enable method chaining
func (e *img) Sizes(value string) *img {
	e.Attribute("sizes", value)
	return e
}

// SizesIf conditionally sets the "sizes" attribute
// Only sets the attribute if the condition is true
func (e *img) SizesIf(condition bool, value string) *img {
	if condition {
		e.Attribute("sizes", value)
	}
	return e
}

// Crossorigin sets the "crossorigin" attribute
// Returns the element itself to enable method chaining
func (e *img) Crossorigin(value string) *img {
	e.Attribute("crossorigin", value)
	return e
}

// CrossoriginIf conditionally sets the "crossorigin" attribute
// Only sets the attribute if the condition is true
func (e *img) CrossoriginIf(condition bool, value string) *img {
	if condition {
		e.Attribute("crossorigin", value)
	}
	return e
}

// Decoding sets the "decoding" attribute
// Returns the element itself to enable method chaining
func (e *img) Decoding(value string) *img {
	e.Attribute("decoding", value)
	return e
}

// DecodingIf conditionally sets the "decoding" attribute
// Only sets the attribute if the condition is true
func (e *img) DecodingIf(condition bool, value string) *img {
	if condition {
		e.Attribute("decoding", value)
	}
	return e
}

// Ismap sets the "ismap" attribute
// Returns the element itself to enable method chaining
func (e *img) Ismap(value string) *img {
	e.Attribute("ismap", value)
	return e
}

// IsmapIf conditionally sets the "ismap" attribute
// Only sets the attribute if the condition is true
func (e *img) IsmapIf(condition bool, value string) *img {
	if condition {
		e.Attribute("ismap", value)
	}
	return e
}

// Usemap sets the "usemap" attribute
// Returns the element itself to enable method chaining
func (e *img) Usemap(value string) *img {
	e.Attribute("usemap", value)
	return e
}

// UsemapIf conditionally sets the "usemap" attribute
// Only sets the attribute if the condition is true
func (e *img) UsemapIf(condition bool, value string) *img {
	if condition {
		e.Attribute("usemap", value)
	}
	return e
}

// Input represents the <input> HTML element
type input struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Input creates a new input element
// Allows optional child nodes to be passed during creation
func Input(children ...Node) *input {
	return &input{NewTag("input", true, children)}
}

// Name sets the "name" attribute
// Returns the element itself to enable method chaining
func (e *input) Name(values ...string) *input {
	e.Attribute("name", strings.Join(values, " "))
	return e
}

// NameIf conditionally sets the "name" attribute
// Only sets the attribute if the condition is true
func (e *input) NameIf(condition bool, value string) *input {
	e.Attribute("name", value)
	return e
}

// Id sets the "id" attribute
// Returns the element itself to enable method chaining
func (e *input) Id(values ...string) *input {
	e.Attribute("id", strings.Join(values, " "))
	return e
}

// IdIf conditionally sets the "id" attribute
// Only sets the attribute if the condition is true
func (e *input) IdIf(condition bool, value string) *input {
	e.Attribute("id", value)
	return e
}

// Type sets the "type" attribute
// Returns the element itself to enable method chaining
func (e *input) Type(value string) *input {
	e.Attribute("type", value)
	return e
}

// TypeIf conditionally sets the "type" attribute
// Only sets the attribute if the condition is true
func (e *input) TypeIf(condition bool, value string) *input {
	if condition {
		e.Attribute("type", value)
	}
	return e
}

// Placeholder sets the "placeholder" attribute
// Returns the element itself to enable method chaining
func (e *input) Placeholder(value string) *input {
	e.Attribute("placeholder", value)
	return e
}

// PlaceholderIf conditionally sets the "placeholder" attribute
// Only sets the attribute if the condition is true
func (e *input) PlaceholderIf(condition bool, value string) *input {
	if condition {
		e.Attribute("placeholder", value)
	}
	return e
}

// Required sets the "required" attribute
// Returns the element itself to enable method chaining
func (e *input) Required(value string) *input {
	e.Attribute("required", value)
	return e
}

// RequiredIf conditionally sets the "required" attribute
// Only sets the attribute if the condition is true
func (e *input) RequiredIf(condition bool, value string) *input {
	if condition {
		e.Attribute("required", value)
	}
	return e
}

// Readonly sets the "readonly" attribute
// Returns the element itself to enable method chaining
func (e *input) Readonly(value string) *input {
	e.Attribute("readonly", value)
	return e
}

// ReadonlyIf conditionally sets the "readonly" attribute
// Only sets the attribute if the condition is true
func (e *input) ReadonlyIf(condition bool, value string) *input {
	if condition {
		e.Attribute("readonly", value)
	}
	return e
}

// Checked sets the "checked" attribute
// Returns the element itself to enable method chaining
func (e *input) Checked(value string) *input {
	e.Attribute("checked", value)
	return e
}

// CheckedIf conditionally sets the "checked" attribute
// Only sets the attribute if the condition is true
func (e *input) CheckedIf(condition bool, value string) *input {
	if condition {
		e.Attribute("checked", value)
	}
	return e
}

// Min sets the "min" attribute
// Returns the element itself to enable method chaining
func (e *input) Min(value string) *input {
	e.Attribute("min", value)
	return e
}

// MinIf conditionally sets the "min" attribute
// Only sets the attribute if the condition is true
func (e *input) MinIf(condition bool, value string) *input {
	if condition {
		e.Attribute("min", value)
	}
	return e
}

// Max sets the "max" attribute
// Returns the element itself to enable method chaining
func (e *input) Max(value string) *input {
	e.Attribute("max", value)
	return e
}

// MaxIf conditionally sets the "max" attribute
// Only sets the attribute if the condition is true
func (e *input) MaxIf(condition bool, value string) *input {
	if condition {
		e.Attribute("max", value)
	}
	return e
}

// Pattern sets the "pattern" attribute
// Returns the element itself to enable method chaining
func (e *input) Pattern(value string) *input {
	e.Attribute("pattern", value)
	return e
}

// PatternIf conditionally sets the "pattern" attribute
// Only sets the attribute if the condition is true
func (e *input) PatternIf(condition bool, value string) *input {
	if condition {
		e.Attribute("pattern", value)
	}
	return e
}

// Ins represents the <ins> HTML element
type ins struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Ins creates a new ins element
// Allows optional child nodes to be passed during creation
func Ins(children ...Node) *ins {
	return &ins{NewTag("ins", false, children)}
}

// Kbd represents the <kbd> HTML element
type kbd struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Kbd creates a new kbd element
// Allows optional child nodes to be passed during creation
func Kbd(children ...Node) *kbd {
	return &kbd{NewTag("kbd", false, children)}
}

// Label represents the <label> HTML element
type label struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Label creates a new label element
// Allows optional child nodes to be passed during creation
func Label(children ...Node) *label {
	return &label{NewTag("label", false, children)}
}

// For sets the "for" attribute
// Returns the element itself to enable method chaining
func (e *label) For(value string) *label {
	e.Attribute("for", value)
	return e
}

// ForIf conditionally sets the "for" attribute
// Only sets the attribute if the condition is true
func (e *label) ForIf(condition bool, value string) *label {
	if condition {
		e.Attribute("for", value)
	}
	return e
}

// Legend represents the <legend> HTML element
type legend struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Legend creates a new legend element
// Allows optional child nodes to be passed during creation
func Legend(children ...Node) *legend {
	return &legend{NewTag("legend", false, children)}
}

// Li represents the <li> HTML element
type li struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Li creates a new li element
// Allows optional child nodes to be passed during creation
func Li(children ...Node) *li {
	return &li{NewTag("li", false, children)}
}

// Link represents the <link> HTML element
type link struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Rel sets the "rel" attribute
// Returns the element itself to enable method chaining
func (e *link) Rel(value string) *link {
	e.Attribute("rel", value)
	return e
}

// RelIf conditionally sets the "rel" attribute
// Only sets the attribute if the condition is true
func (e *link) RelIf(condition bool, value string) *link {
	if condition {
		e.Attribute("rel", value)
	}
	return e
}

// Href sets the "href" attribute
// Returns the element itself to enable method chaining
func (e *link) Href(value string) *link {
	e.Attribute("href", value)
	return e
}

// HrefIf conditionally sets the "href" attribute
// Only sets the attribute if the condition is true
func (e *link) HrefIf(condition bool, value string) *link {
	if condition {
		e.Attribute("href", value)
	}
	return e
}

// Link creates a new link element
// Allows optional child nodes to be passed during creation
func Link(children ...Node) *link {
	return &link{NewTag("link", true, children)}
}

// Integrity sets the "integrity" attribute
// Returns the element itself to enable method chaining
func (e *link) Integrity(value string) *link {
	e.Attribute("integrity", value)
	return e
}

// IntegrityIf conditionally sets the "integrity" attribute
// Only sets the attribute if the condition is true
func (e *link) IntegrityIf(condition bool, value string) *link {
	if condition {
		e.Attribute("integrity", value)
	}
	return e
}

// Main represents the <main> HTML element
type main struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Main creates a new main element
// Allows optional child nodes to be passed during creation
func Main(children ...Node) *main {
	return &main{NewTag("main", false, children)}
}

// mapEl_ represents the <map> HTML element
type mapEl_ struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Map creates a new map element
// Allows optional child nodes to be passed during creation
func MapEl(children ...Node) *mapEl_ {
	return &mapEl_{NewTag("map", false, children)}
}

// Mark represents the <mark> HTML element
type mark struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Mark creates a new mark element
// Allows optional child nodes to be passed during creation
func Mark(children ...Node) *mark {
	return &mark{NewTag("mark", false, children)}
}

// Meta represents the <meta> HTML element
type meta struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Meta creates a new meta element
// Allows optional child nodes to be passed during creation
func Meta(children ...Node) *meta {
	return &meta{NewTag("meta", true, children)}
}

// Name sets the "name" attribute
// Returns the element itself to enable method chaining
func (e *meta) Name(value string) *meta {
	e.Attribute("name", value)
	return e
}

// NameIf conditionally sets the "name" attribute
// Only sets the attribute if the condition is true
func (e *meta) NameIf(condition bool, value string) *meta {
	if condition {
		e.Attribute("name", value)
	}
	return e
}

// Content sets the "content" attribute
// Returns the element itself to enable method chaining
func (e *meta) Content(value string) *meta {
	e.Attribute("content", value)
	return e
}

// ContentIf conditionally sets the "content" attribute
// Only sets the attribute if the condition is true
func (e *meta) ContentIf(condition bool, value string) *meta {
	if condition {
		e.Attribute("content", value)
	}
	return e
}

// Charset sets the "charset" attribute
// Returns the element itself to enable method chaining
func (e *meta) Charset(value string) *meta {
	e.Attribute("charset", value)
	return e
}

// CharsetIf conditionally sets the "charset" attribute
// Only sets the attribute if the condition is true
func (e *meta) CharsetIf(condition bool, value string) *meta {
	if condition {
		e.Attribute("charset", value)
	}
	return e
}

// HttpEquiv sets the "http-equiv" attribute
// Returns the element itself to enable method chaining
func (e *meta) HttpEquiv(value string) *meta {
	e.Attribute("http-equiv", value)
	return e
}

// HttpEquivIf conditionally sets the "http-equiv" attribute
// Only sets the attribute if the condition is true
func (e *meta) HttpEquivIf(condition bool, value string) *meta {
	if condition {
		e.Attribute("http-equiv", value)
	}
	return e
}

// Meter represents the <meter> HTML element
type meter struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Meter creates a new meter element
// Allows optional child nodes to be passed during creation
func Meter(children ...Node) *meter {
	return &meter{NewTag("meter", false, children)}
}

// Low sets the "low" attribute
// Returns the element itself to enable method chaining
func (e *meter) Low(value string) *meter {
	e.Attribute("low", value)
	return e
}

// LowIf conditionally sets the "low" attribute
// Only sets the attribute if the condition is true
func (e *meter) LowIf(condition bool, value string) *meter {
	if condition {
		e.Attribute("low", value)
	}
	return e
}

// High sets the "high" attribute
// Returns the element itself to enable method chaining
func (e *meter) High(value string) *meter {
	e.Attribute("high", value)
	return e
}

// HighIf conditionally sets the "high" attribute
// Only sets the attribute if the condition is true
func (e *meter) HighIf(condition bool, value string) *meter {
	if condition {
		e.Attribute("high", value)
	}
	return e
}

// Optimum sets the "optimum" attribute
// Returns the element itself to enable method chaining
func (e *meter) Optimum(value string) *meter {
	e.Attribute("optimum", value)
	return e
}

// OptimumIf conditionally sets the "optimum" attribute
// Only sets the attribute if the condition is true
func (e *meter) OptimumIf(condition bool, value string) *meter {
	if condition {
		e.Attribute("optimum", value)
	}
	return e
}

// Nav represents the <nav> HTML element
type nav struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Nav creates a new nav element
// Allows optional child nodes to be passed during creation
func Nav(children ...Node) *nav {
	return &nav{NewTag("nav", false, children)}
}

// Noscript represents the <noscript> HTML element
type noscript struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Noscript creates a new noscript element
// Allows optional child nodes to be passed during creation
func Noscript(children ...Node) *noscript {
	return &noscript{NewTag("noscript", false, children)}
}

// Object represents the <object> HTML element
type object struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Object creates a new object element
// Allows optional child nodes to be passed during creation
func Object(children ...Node) *object {
	return &object{NewTag("object", false, children)}
}

// Ol represents the <ol> HTML element
type ol struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Ol creates a new ol element
// Allows optional child nodes to be passed during creation
func Ol(children ...Node) *ol {
	return &ol{NewTag("ol", false, children)}
}

// Start sets the "start" attribute
// Returns the element itself to enable method chaining
func (e *ol) Start(value string) *ol {
	e.Attribute("start", value)
	return e
}

// StartIf conditionally sets the "start" attribute
// Only sets the attribute if the condition is true
func (e *ol) StartIf(condition bool, value string) *ol {
	if condition {
		e.Attribute("start", value)
	}
	return e
}

// Reversed sets the "reversed" attribute
// Returns the element itself to enable method chaining
func (e *ol) Reversed(value string) *ol {
	e.Attribute("reversed", value)
	return e
}

// ReversedIf conditionally sets the "reversed" attribute
// Only sets the attribute if the condition is true
func (e *ol) ReversedIf(condition bool, value string) *ol {
	if condition {
		e.Attribute("reversed", value)
	}
	return e
}

// Optgroup represents the <optgroup> HTML element
type optgroup struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Optgroup creates a new optgroup element
// Allows optional child nodes to be passed during creation
func Optgroup(children ...Node) *optgroup {
	return &optgroup{NewTag("optgroup", false, children)}
}

// Label sets the "label" attribute
// Returns the element itself to enable method chaining
func (e *optgroup) Label(value string) *optgroup {
	e.Attribute("label", value)
	return e
}

// LabelIf conditionally sets the "label" attribute
// Only sets the attribute if the condition is true
func (e *optgroup) LabelIf(condition bool, value string) *optgroup {
	if condition {
		e.Attribute("label", value)
	}
	return e
}

// Option represents the <option> HTML element
type option struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Option creates a new option element
// Allows optional child nodes to be passed during creation
func Option(children ...Node) *option {
	return &option{NewTag("option", false, children)}
}

// Selected sets the "selected" attribute
// Returns the element itself to enable method chaining
func (e *option) Selected(value string) *option {
	e.Attribute("selected", value)
	return e
}

// SelectedIf conditionally sets the "selected" attribute
// Only sets the attribute if the condition is true
func (e *option) SelectedIf(condition bool, value string) *option {
	if condition {
		e.Attribute("selected", value)
	}
	return e
}

// Output represents the <output> HTML element
type output struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Output creates a new output element
// Allows optional child nodes to be passed during creation
func Output(children ...Node) *output {
	return &output{NewTag("output", false, children)}
}

// P represents the <p> HTML element
type p struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// P creates a new p element
// Allows optional child nodes to be passed during creation
func P(children ...Node) *p {
	return &p{NewTag("p", false, children)}
}

// Param represents the <param> HTML element
type param struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Param creates a new param element
// Allows optional child nodes to be passed during creation
func Param(children ...Node) *param {
	return &param{NewTag("param", true, children)}
}

// Picture represents the <picture> HTML element
type picture struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Picture creates a new picture element
// Allows optional child nodes to be passed during creation
func Picture(children ...Node) *picture {
	return &picture{NewTag("picture", false, children)}
}

// Pre represents the <pre> HTML element
type pre struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Pre creates a new pre element
// Allows optional child nodes to be passed during creation
func Pre(children ...Node) *pre {
	return &pre{NewTag("pre", false, children)}
}

// Progress represents the <progress> HTML element
type progress struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Progress creates a new progress element
// Allows optional child nodes to be passed during creation
func Progress(children ...Node) *progress {
	return &progress{NewTag("progress", false, children)}
}

// Q represents the <q> HTML element
type q struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Q creates a new q element
// Allows optional child nodes to be passed during creation
func Q(children ...Node) *q {
	return &q{NewTag("q", false, children)}
}

// Rp represents the <rp> HTML element
type rp struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Rp creates a new rp element
// Allows optional child nodes to be passed during creation
func Rp(children ...Node) *rp {
	return &rp{NewTag("rp", false, children)}
}

// Rt represents the <rt> HTML element
type rt struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Rt creates a new rt element
// Allows optional child nodes to be passed during creation
func Rt(children ...Node) *rt {
	return &rt{NewTag("rt", false, children)}
}

// Ruby represents the <ruby> HTML element
type ruby struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Ruby creates a new ruby element
// Allows optional child nodes to be passed during creation
func Ruby(children ...Node) *ruby {
	return &ruby{NewTag("ruby", false, children)}
}

// S represents the <s> HTML element
type s struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// S creates a new s element
// Allows optional child nodes to be passed during creation
func S(children ...Node) *s {
	return &s{NewTag("s", false, children)}
}

// Samp represents the <samp> HTML element
type samp struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Samp creates a new samp element
// Allows optional child nodes to be passed during creation
func Samp(children ...Node) *samp {
	return &samp{NewTag("samp", false, children)}
}

// Script represents the <script> HTML element
type script struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Script creates a new script element
// Allows optional child nodes to be passed during creation
func Script(children ...Node) *script {
	return &script{NewTag("script", false, children)}
}

// Async sets the "async" attribute
// Returns the element itself to enable method chaining
func (e *script) Async(value string) *script {
	e.Attribute("async", value)
	return e
}

// AsyncIf conditionally sets the "async" attribute
// Only sets the attribute if the condition is true
func (e *script) AsyncIf(condition bool, value string) *script {
	if condition {
		e.Attribute("async", value)
	}
	return e
}

// Defer sets the "defer" attribute
// Returns the element itself to enable method chaining
func (e *script) Defer(value string) *script {
	e.Attribute("defer", value)
	return e
}

// DeferIf conditionally sets the "defer" attribute
// Only sets the attribute if the condition is true
func (e *script) DeferIf(condition bool, value string) *script {
	if condition {
		e.Attribute("defer", value)
	}
	return e
}

// Nomodule sets the "nomodule" attribute
// Returns the element itself to enable method chaining
func (e *script) Nomodule(value string) *script {
	e.Attribute("nomodule", value)
	return e
}

// NomoduleIf conditionally sets the "nomodule" attribute
// Only sets the attribute if the condition is true
func (e *script) NomoduleIf(condition bool, value string) *script {
	if condition {
		e.Attribute("nomodule", value)
	}
	return e
}

// Type sets the "type" attribute
// Returns the element itself to enable method chaining
func (e *script) Type(value string) *script {
	e.Attribute("type", value)
	return e
}

// TypeIf conditionally sets the "type" attribute
// Only sets the attribute if the condition is true
func (e *script) TypeIf(condition bool, value string) *script {
	if condition {
		e.Attribute("type", value)
	}
	return e
}

// Src sets the "src" attribute
// Returns the element itself to enable method chaining
func (e *script) Src(value string) *script {
	e.Attribute("src", value)
	return e
}

// SrcIf conditionally sets the "src" attribute
// Only sets the attribute if the condition is true
func (e *script) SrcIf(condition bool, value string) *script {
	if condition {
		e.Attribute("Src", value)
	}
	return e
}

// Section represents the <section> HTML element
type section struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Section creates a new section element
// Allows optional child nodes to be passed during creation
func Section(children ...Node) *section {
	return &section{NewTag("section", false, children)}
}

// Select represents the <select> HTML element
type select_ struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Select creates a new select element
// Allows optional child nodes to be passed during creation
func Select(children ...Node) *select_ {
	return &select_{NewTag("select", false, children)}
}

// Multiple sets the "multiple" attribute
// Returns the element itself to enable method chaining
func (e *select_) Multiple(value string) *select_ {
	e.Attribute("multiple", value)
	return e
}

// MultipleIf conditionally sets the "multiple" attribute
// Only sets the attribute if the condition is true
func (e *select_) MultipleIf(condition bool, value string) *select_ {
	if condition {
		e.Attribute("multiple", value)
	}
	return e
}

// Size sets the "size" attribute
// Returns the element itself to enable method chaining
func (e *select_) Size(value string) *select_ {
	e.Attribute("size", value)
	return e
}

// SizeIf conditionally sets the "size" attribute
// Only sets the attribute if the condition is true
func (e *select_) SizeIf(condition bool, value string) *select_ {
	if condition {
		e.Attribute("size", value)
	}
	return e
}

// Small represents the <small> HTML element
type small struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Small creates a new small element
// Allows optional child nodes to be passed during creation
func Small(children ...Node) *small {
	return &small{NewTag("small", false, children)}
}

// Source represents the <source> HTML element
type source struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Source creates a new source element
// Allows optional child nodes to be passed during creation
func Source(children ...Node) *source {
	return &source{NewTag("source", true, children)}
}

// Span represents the <span> HTML element
type span struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Span creates a new span element
// Allows optional child nodes to be passed during creation
func Span(children ...Node) *span {
	return &span{NewTag("span", false, children)}
}

// Strong represents the <strong> HTML element
type strong struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Strong creates a new strong element
// Allows optional child nodes to be passed during creation
func Strong(children ...Node) *strong {
	return &strong{NewTag("strong", false, children)}
}

// Style represents the <style> HTML element
type style struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Style creates a new style element
// Allows optional child nodes to be passed during creation
func Style(children ...Node) *style {
	return &style{NewTag("style", false, children)}
}

// Sub represents the <sub> HTML element
type sub struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Sub creates a new sub element
// Allows optional child nodes to be passed during creation
func Sub(children ...Node) *sub {
	return &sub{NewTag("sub", false, children)}
}

// Summary represents the <summary> HTML element
type summary struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Summary creates a new summary element
// Allows optional child nodes to be passed during creation
func Summary(children ...Node) *summary {
	return &summary{NewTag("summary", false, children)}
}

// Sup represents the <sup> HTML element
type sup struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Sup creates a new sup element
// Allows optional child nodes to be passed during creation
func Sup(children ...Node) *sup {
	return &sup{NewTag("sup", false, children)}
}

// svg represents the <svg> HTML element
type svg struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// SVG creates a new svg element
// Allows optional child nodes to be passed during creation
func SVG(children ...Node) *svg {
	return &svg{NewTag("svg", false, children)}
}

// ViewBox sets the "viewBox" attribute
// Returns the element itself to enable method chaining
func (e *svg) ViewBox(value string) *svg {
	e.Attribute("viewBox", value)
	return e
}

// ViewBoxIf conditionally sets the "viewBox" attribute
// Only sets the attribute if the condition is true
func (e *svg) ViewBoxIf(condition bool, value string) *svg {
	if condition {
		e.Attribute("viewBox", value)
	}
	return e
}

// PreserveAspectRatio sets the "preserveAspectRatio" attribute
// Returns the element itself to enable method chaining
func (e *svg) PreserveAspectRatio(value string) *svg {
	e.Attribute("preserveAspectRatio", value)
	return e
}

// PreserveAspectRatioIf conditionally sets the "preserveAspectRatio" attribute
// Only sets the attribute if the condition is true
func (e *svg) PreserveAspectRatioIf(condition bool, value string) *svg {
	if condition {
		e.Attribute("preserveAspectRatio", value)
	}
	return e
}

// Xmlns sets the "xmlns" attribute
// Returns the element itself to enable method chaining
func (e *svg) Xmlns(value string) *svg {
	e.Attribute("xmlns", value)
	return e
}

// XmlnsIf conditionally sets the "xmlns" attribute
// Only sets the attribute if the condition is true
func (e *svg) XmlnsIf(condition bool, value string) *svg {
	if condition {
		e.Attribute("xmlns", value)
	}
	return e
}

// Version sets the "version" attribute
// Returns the element itself to enable method chaining
func (e *svg) Version(value string) *svg {
	e.Attribute("version", value)
	return e
}

// VersionIf conditionally sets the "version" attribute
// Only sets the attribute if the condition is true
func (e *svg) VersionIf(condition bool, value string) *svg {
	if condition {
		e.Attribute("version", value)
	}
	return e
}

// Table represents the <table> HTML element
type table struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Table creates a new table element
// Allows optional child nodes to be passed during creation
func Table(children ...Node) *table {
	return &table{NewTag("table", false, children)}
}

// Border sets the "border" attribute
// Returns the element itself to enable method chaining
func (e *table) Border(value string) *table {
	e.Attribute("border", value)
	return e
}

// BorderIf conditionally sets the "border" attribute
// Only sets the attribute if the condition is true
func (e *table) BorderIf(condition bool, value string) *table {
	if condition {
		e.Attribute("border", value)
	}
	return e
}

// Cellpadding sets the "cellpadding" attribute
// Returns the element itself to enable method chaining
func (e *table) Cellpadding(value string) *table {
	e.Attribute("cellpadding", value)
	return e
}

// CellpaddingIf conditionally sets the "cellpadding" attribute
// Only sets the attribute if the condition is true
func (e *table) CellpaddingIf(condition bool, value string) *table {
	if condition {
		e.Attribute("cellpadding", value)
	}
	return e
}

// Cellspacing sets the "cellspacing" attribute
// Returns the element itself to enable method chaining
func (e *table) Cellspacing(value string) *table {
	e.Attribute("cellspacing", value)
	return e
}

// CellspacingIf conditionally sets the "cellspacing" attribute
// Only sets the attribute if the condition is true
func (e *table) CellspacingIf(condition bool, value string) *table {
	if condition {
		e.Attribute("cellspacing", value)
	}
	return e
}

// Tbody represents the <tbody> HTML element
type tbody struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Tbody creates a new tbody element
// Allows optional child nodes to be passed during creation
func Tbody(children ...Node) *tbody {
	return &tbody{NewTag("tbody", false, children)}
}

// Td represents the <td> HTML element
type td struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Td creates a new td element
// Allows optional child nodes to be passed during creation
func Td(children ...Node) *td {
	return &td{NewTag("td", false, children)}
}

// Colspan sets the "colspan" attribute
// Returns the element itself to enable method chaining
func (e *td) Colspan(value string) *td {
	e.Attribute("colspan", value)
	return e
}

// ColspanIf conditionally sets the "colspan" attribute
// Only sets the attribute if the condition is true
func (e *td) ColspanIf(condition bool, value string) *td {
	if condition {
		e.Attribute("colspan", value)
	}
	return e
}

// Rowspan sets the "rowspan" attribute
// Returns the element itself to enable method chaining
func (e *td) Rowspan(value string) *td {
	e.Attribute("rowspan", value)
	return e
}

// RowspanIf conditionally sets the "rowspan" attribute
// Only sets the attribute if the condition is true
func (e *td) RowspanIf(condition bool, value string) *td {
	if condition {
		e.Attribute("rowspan", value)
	}
	return e
}

// Headers sets the "headers" attribute
// Returns the element itself to enable method chaining
func (e *td) Headers(value string) *td {
	e.Attribute("headers", value)
	return e
}

// HeadersIf conditionally sets the "headers" attribute
// Only sets the attribute if the condition is true
func (e *td) HeadersIf(condition bool, value string) *td {
	if condition {
		e.Attribute("headers", value)
	}
	return e
}

// Template represents the <template> HTML element
type template struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Template creates a new template element
// Allows optional child nodes to be passed during creation
func Template(children ...Node) *template {
	return &template{NewTag("template", false, children)}
}

// Textarea represents the <textarea> HTML element
type textarea struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Textarea creates a new textarea element
// Allows optional child nodes to be passed during creation
func Textarea(children ...Node) *textarea {
	return &textarea{NewTag("textarea", false, children)}
}

// Rows sets the "rows" attribute
// Returns the element itself to enable method chaining
func (e *textarea) Rows(value string) *textarea {
	e.Attribute("rows", value)
	return e
}

// RowsIf conditionally sets the "rows" attribute
// Only sets the attribute if the condition is true
func (e *textarea) RowsIf(condition bool, value string) *textarea {
	if condition {
		e.Attribute("rows", value)
	}
	return e
}

// Cols sets the "cols" attribute
// Returns the element itself to enable method chaining
func (e *textarea) Cols(value string) *textarea {
	e.Attribute("cols", value)
	return e
}

// ColsIf conditionally sets the "cols" attribute
// Only sets the attribute if the condition is true
func (e *textarea) ColsIf(condition bool, value string) *textarea {
	if condition {
		e.Attribute("cols", value)
	}
	return e
}

// Maxlength sets the "maxlength" attribute
// Returns the element itself to enable method chaining
func (e *textarea) Maxlength(value string) *textarea {
	e.Attribute("maxlength", value)
	return e
}

// MaxlengthIf conditionally sets the "maxlength" attribute
// Only sets the attribute if the condition is true
func (e *textarea) MaxlengthIf(condition bool, value string) *textarea {
	if condition {
		e.Attribute("maxlength", value)
	}
	return e
}

// Minlength sets the "minlength" attribute
// Returns the element itself to enable method chaining
func (e *textarea) Minlength(value string) *textarea {
	e.Attribute("minlength", value)
	return e
}

// MinlengthIf conditionally sets the "minlength" attribute
// Only sets the attribute if the condition is true
func (e *textarea) MinlengthIf(condition bool, value string) *textarea {
	if condition {
		e.Attribute("minlength", value)
	}
	return e
}

// Wrap sets the "wrap" attribute
// Returns the element itself to enable method chaining
func (e *textarea) Wrap(value string) *textarea {
	e.Attribute("wrap", value)
	return e
}

// WrapIf conditionally sets the "wrap" attribute
// Only sets the attribute if the condition is true
func (e *textarea) WrapIf(condition bool, value string) *textarea {
	if condition {
		e.Attribute("wrap", value)
	}
	return e
}

// Tfoot represents the <tfoot> HTML element
type tfoot struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Tfoot creates a new tfoot element
// Allows optional child nodes to be passed during creation
func Tfoot(children ...Node) *tfoot {
	return &tfoot{NewTag("tfoot", false, children)}
}

// Th represents the <th> HTML element
type th struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Th creates a new th element
// Allows optional child nodes to be passed during creation
func Th(children ...Node) *th {
	return &th{NewTag("th", false, children)}
}

// Scope sets the "scope" attribute
// Returns the element itself to enable method chaining
func (e *th) Scope(value string) *th {
	e.Attribute("scope", value)
	return e
}

// ScopeIf conditionally sets the "scope" attribute
// Only sets the attribute if the condition is true
func (e *th) ScopeIf(condition bool, value string) *th {
	if condition {
		e.Attribute("scope", value)
	}
	return e
}

// Thead represents the <thead> HTML element
type thead struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Thead creates a new thead element
// Allows optional child nodes to be passed during creation
func Thead(children ...Node) *thead {
	return &thead{NewTag("thead", false, children)}
}

// Time represents the <time> HTML element
type time struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Time creates a new time element
// Allows optional child nodes to be passed during creation
func Time(children ...Node) *time {
	return &time{NewTag("time", false, children)}
}

// Datetime sets the "datetime" attribute
// Returns the element itself to enable method chaining
func (e *time) Datetime(value string) *time {
	e.Attribute("datetime", value)
	return e
}

// DatetimeIf conditionally sets the "datetime" attribute
// Only sets the attribute if the condition is true
func (e *time) DatetimeIf(condition bool, value string) *time {
	if condition {
		e.Attribute("datetime", value)
	}
	return e
}

// Title represents the <title> HTML element
type title struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Title creates a new title element
// Allows optional child nodes to be passed during creation
func Title(children ...Node) *title {
	return &title{NewTag("title", false, children)}
}

// Tr represents the <tr> HTML element
type tr struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Tr creates a new tr element
// Allows optional child nodes to be passed during creation
func Tr(children ...Node) *tr {
	return &tr{NewTag("tr", false, children)}
}

// Track represents the <track> HTML element
type track struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Track creates a new track element
// Allows optional child nodes to be passed during creation
func Track(children ...Node) *track {
	return &track{NewTag("track", true, children)}
}

// U represents the <u> HTML element
type u struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// U creates a new u element
// Allows optional child nodes to be passed during creation
func U(children ...Node) *u {
	return &u{NewTag("u", false, children)}
}

// Ul represents the <ul> HTML element
type ul struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Ul creates a new ul element
// Allows optional child nodes to be passed during creation
func Ul(children ...Node) *ul {
	return &ul{NewTag("ul", false, children)}
}

// Var represents the <var> HTML element
type var_ struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Var creates a new var element
// Allows optional child nodes to be passed during creation
func Var(children ...Node) *var_ {
	return &var_{NewTag("var", false, children)}
}

// Video represents the <video> HTML element
type video struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Video creates a new video element
// Allows optional child nodes to be passed during creation
func Video(children ...Node) *video {
	return &video{NewTag("video", false, children)}
}

// Poster sets the "poster" attribute
// Returns the element itself to enable method chaining
func (e *video) Poster(value string) *video {
	e.Attribute("poster", value)
	return e
}

// PosterIf conditionally sets the "poster" attribute
// Only sets the attribute if the condition is true
func (e *video) PosterIf(condition bool, value string) *video {
	if condition {
		e.Attribute("poster", value)
	}
	return e
}

// Wbr represents the <wbr> HTML element
type wbr struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Wbr creates a new wbr element
// Allows optional child nodes to be passed during creation
func Wbr(children ...Node) *wbr {
	return &wbr{NewTag("wbr", true, children)}
}

// Circle represents the <circle> HTML element
type circle struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Circle creates a new circle element
// Allows optional child nodes to be passed during creation
func Circle(children ...Node) *circle {
	return &circle{NewTag("circle", false, children)}
}

// Cx sets the "cx" attribute
// Returns the element itself to enable method chaining
func (e *circle) Cx(value string) *circle {
	e.Attribute("cx", value)
	return e
}

// CxIf conditionally sets the "cx" attribute
// Only sets the attribute if the condition is true
func (e *circle) CxIf(condition bool, value string) *circle {
	if condition {
		e.Attribute("cx", value)
	}
	return e
}

// Cy sets the "cy" attribute
// Returns the element itself to enable method chaining
func (e *circle) Cy(value string) *circle {
	e.Attribute("cy", value)
	return e
}

// CyIf conditionally sets the "cy" attribute
// Only sets the attribute if the condition is true
func (e *circle) CyIf(condition bool, value string) *circle {
	if condition {
		e.Attribute("cy", value)
	}
	return e
}

// R sets the "r" attribute
// Returns the element itself to enable method chaining
func (e *circle) R(value string) *circle {
	e.Attribute("r", value)
	return e
}

// RIf conditionally sets the "r" attribute
// Only sets the attribute if the condition is true
func (e *circle) RIf(condition bool, value string) *circle {
	if condition {
		e.Attribute("r", value)
	}
	return e
}

// Fill sets the "fill" attribute
// Returns the element itself to enable method chaining
func (e *circle) Fill(value string) *circle {
	e.Attribute("fill", value)
	return e
}

// FillIf conditionally sets the "fill" attribute
// Only sets the attribute if the condition is true
func (e *circle) FillIf(condition bool, value string) *circle {
	if condition {
		e.Attribute("fill", value)
	}
	return e
}

// Stroke sets the "stroke" attribute
// Returns the element itself to enable method chaining
func (e *circle) Stroke(value string) *circle {
	e.Attribute("stroke", value)
	return e
}

// StrokeIf conditionally sets the "stroke" attribute
// Only sets the attribute if the condition is true
func (e *circle) StrokeIf(condition bool, value string) *circle {
	if condition {
		e.Attribute("stroke", value)
	}
	return e
}

// Ellipse represents the <ellipse> HTML element
type ellipse struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Ellipse creates a new ellipse element
// Allows optional child nodes to be passed during creation
func Ellipse(children ...Node) *ellipse {
	return &ellipse{NewTag("ellipse", false, children)}
}

// G represents the <g> HTML element
type g struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// G creates a new g element
// Allows optional child nodes to be passed during creation
func G(children ...Node) *g {
	return &g{NewTag("g", false, children)}
}

// Transform sets the "transform" attribute
// Returns the element itself to enable method chaining
func (e *g) Transform(value string) *g {
	e.Attribute("transform", value)
	return e
}

// TransformIf conditionally sets the "transform" attribute
// Only sets the attribute if the condition is true
func (e *g) TransformIf(condition bool, value string) *g {
	if condition {
		e.Attribute("transform", value)
	}
	return e
}

// Line represents the <line> HTML element
type line struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Line creates a new line element
// Allows optional child nodes to be passed during creation
func Line(children ...Node) *line {
	return &line{NewTag("line", false, children)}
}

// X1 sets the "x1" attribute
// Returns the element itself to enable method chaining
func (e *line) X1(value string) *line {
	e.Attribute("x1", value)
	return e
}

// X1If conditionally sets the "x1" attribute
// Only sets the attribute if the condition is true
func (e *line) X1If(condition bool, value string) *line {
	if condition {
		e.Attribute("x1", value)
	}
	return e
}

// Y1 sets the "y1" attribute
// Returns the element itself to enable method chaining
func (e *line) Y1(value string) *line {
	e.Attribute("y1", value)
	return e
}

// Y1If conditionally sets the "y1" attribute
// Only sets the attribute if the condition is true
func (e *line) Y1If(condition bool, value string) *line {
	if condition {
		e.Attribute("y1", value)
	}
	return e
}

// X2 sets the "x2" attribute
// Returns the element itself to enable method chaining
func (e *line) X2(value string) *line {
	e.Attribute("x2", value)
	return e
}

// X2If conditionally sets the "x2" attribute
// Only sets the attribute if the condition is true
func (e *line) X2If(condition bool, value string) *line {
	if condition {
		e.Attribute("x2", value)
	}
	return e
}

// Y2 sets the "y2" attribute
// Returns the element itself to enable method chaining
func (e *line) Y2(value string) *line {
	e.Attribute("y2", value)
	return e
}

// Y2If conditionally sets the "y2" attribute
// Only sets the attribute if the condition is true
func (e *line) Y2If(condition bool, value string) *line {
	if condition {
		e.Attribute("y2", value)
	}
	return e
}

// StrokeWidth sets the "stroke-width" attribute
// Returns the element itself to enable method chaining
func (e *line) StrokeWidth(value string) *line {
	e.Attribute("stroke-width", value)
	return e
}

// StrokeWidthIf conditionally sets the "stroke-width" attribute
// Only sets the attribute if the condition is true
func (e *line) StrokeWidthIf(condition bool, value string) *line {
	if condition {
		e.Attribute("stroke-width", value)
	}
	return e
}

// Path represents the <path> HTML element
type path struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Path creates a new path element
// Allows optional child nodes to be passed during creation
func Path(children ...Node) *path {
	return &path{NewTag("path", false, children)}
}

// D sets the "d" attribute
// Returns the element itself to enable method chaining
func (e *path) D(value string) *path {
	e.Attribute("d", value)
	return e
}

// DIf conditionally sets the "d" attribute
// Only sets the attribute if the condition is true
func (e *path) DIf(condition bool, value string) *path {
	if condition {
		e.Attribute("d", value)
	}
	return e
}

// Polygon represents the <polygon> HTML element
type polygon struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Polygon creates a new polygon element
// Allows optional child nodes to be passed during creation
func Polygon(children ...Node) *polygon {
	return &polygon{NewTag("polygon", false, children)}
}

// Points sets the "points" attribute
// Returns the element itself to enable method chaining
func (e *polygon) Points(value string) *polygon {
	e.Attribute("points", value)
	return e
}

// PointsIf conditionally sets the "points" attribute
// Only sets the attribute if the condition is true
func (e *polygon) PointsIf(condition bool, value string) *polygon {
	if condition {
		e.Attribute("points", value)
	}
	return e
}

// Polyline represents the <polyline> HTML element
type polyline struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Polyline creates a new polyline element
// Allows optional child nodes to be passed during creation
func Polyline(children ...Node) *polyline {
	return &polyline{NewTag("polyline", false, children)}
}

// Rect represents the <rect> HTML element
type rect struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Rect creates a new rect element
// Allows optional child nodes to be passed during creation
func Rect(children ...Node) *rect {
	return &rect{NewTag("rect", false, children)}
}

// X sets the "x" attribute
// Returns the element itself to enable method chaining
func (e *rect) X(value string) *rect {
	e.Attribute("x", value)
	return e
}

// XIf conditionally sets the "x" attribute
// Only sets the attribute if the condition is true
func (e *rect) XIf(condition bool, value string) *rect {
	if condition {
		e.Attribute("x", value)
	}
	return e
}

// Y sets the "y" attribute
// Returns the element itself to enable method chaining
func (e *rect) Y(value string) *rect {
	e.Attribute("y", value)
	return e
}

// YIf conditionally sets the "y" attribute
// Only sets the attribute if the condition is true
func (e *rect) YIf(condition bool, value string) *rect {
	if condition {
		e.Attribute("y", value)
	}
	return e
}

// Rx sets the "rx" attribute
// Returns the element itself to enable method chaining
func (e *rect) Rx(value string) *rect {
	e.Attribute("rx", value)
	return e
}

// RxIf conditionally sets the "rx" attribute
// Only sets the attribute if the condition is true
func (e *rect) RxIf(condition bool, value string) *rect {
	if condition {
		e.Attribute("rx", value)
	}
	return e
}

// Ry sets the "ry" attribute
// Returns the element itself to enable method chaining
func (e *rect) Ry(value string) *rect {
	e.Attribute("ry", value)
	return e
}

// RyIf conditionally sets the "ry" attribute
// Only sets the attribute if the condition is true
func (e *rect) RyIf(condition bool, value string) *rect {
	if condition {
		e.Attribute("ry", value)
	}
	return e
}

// Use represents the <use> HTML element
type use struct {
	// Embeds the base Tag to inherit core HTML element functionality
	*Tag
}

// Use creates a new use element
// Allows optional child nodes to be passed during creation
func Use(children ...Node) *use {
	return &use{NewTag("use", false, children)}
}
