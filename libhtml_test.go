package libhtml_test

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	h "github.com/alexisbcz/libhtml"
)

func render(t *testing.T, n h.Node) string {
	t.Helper()
	s, err := h.RenderString(n)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return s
}

func renderErr(t *testing.T, n h.Node) error {
	t.Helper()
	_, err := h.RenderString(n)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

func TestChildrenOfEveryKind(t *testing.T) {
	var nilEl *h.Element
	got := render(t, h.Div(
		h.Class("a b"), h.ID("x"),
		"text ", 42, " ", 1.5, " ", true,
		nilEl, nil,
		h.Span("span"),
		[]h.Node{h.B("b1"), h.B("b2")},
		[]string{"s1", "s2"},
		h.Group(h.Class("c"), h.I("i")),
		func() h.Node { return h.Em("lazy") },
	))
	want := `<div class="a b c" id="x">text 42 1.5 true<span>span</span><b>b1</b><b>b2</b>s1s2<i>i</i><em>lazy</em></div>`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestBuilderAndChildrenStylesMatch(t *testing.T) {
	a := render(t, h.A(h.Href("/x"), h.Class("link"), "go"))
	b := render(t, h.A().Href("/x").Class("link").Text("go"))
	if a != b {
		t.Fatalf("%s != %s", a, b)
	}
}

func TestUnsupportedChild(t *testing.T) {
	err := renderErr(t, h.Div(struct{}{}))
	if !strings.Contains(err.Error(), "unsupported child") {
		t.Fatal(err)
	}
}

func TestControlFlow(t *testing.T) {
	type user struct{ Name string }
	var nobody *user
	items := []string{"x", "y"}
	got := render(t, h.Div(
		h.If(true, h.Class("on")),
		h.If(false, h.Class("off")),
		h.Unless(false, "unless "),
		h.Iff(nobody != nil, func() h.Node { return h.Text(nobody.Name) }), // must not panic
		h.IfElse(false, "a", "b"),
		h.IfElseF(true, func() h.Node { return h.Text("c") }, func() h.Node { return h.Text(nobody.Name) }),
		h.Class(h.Ternary(true, "t", "f")),
		h.Ul(h.MapIdx(items, func(i int, s string) h.Node { return h.Li(i, ":", s) })),
		h.MapKV(map[string]int{"b": 2, "a": 1}, func(k string, v int) h.Node { return h.Text(k) }),
		h.Range(3, func(i int) any { return i }),
		h.Map(h.Filter([]int{1, 2, 3, 4}, func(i int) bool { return i%2 == 0 }), func(i int) any { return i }),
		h.Switch(2).Case(1, "one").Case(2, h.Data("n", "two")).Default("many"),
		h.Switch("z").Case("a", "A").Default("default"),
	))
	want := `<div class="on t" data-n="two">unless bc<ul><li>0:x</li><li>1:y</li></ul>ab01224default</div>`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestDocumentAndVoid(t *testing.T) {
	got := render(t, h.Doctype(h.Html(h.Lang("en"), h.Head(h.Meta(h.Charset("utf-8"))), h.Body(h.Br(), h.Input(h.Disabled())))))
	want := `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"></head><body><br><input disabled></body></html>`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if err := renderErr(t, h.Br("child")); !h.IsSecurityError(err) {
		t.Fatal(err)
	}
}

// --- XSS -----------------------------------------------------------------------

func TestTextAndAttributeEscaping(t *testing.T) {
	payload := `"><script>alert('x')</script>&`
	got := render(t, h.Div(h.TitleAttr(payload), h.Data("v", payload), h.Class(payload), payload))
	if strings.Contains(got, "<script>") || strings.Contains(got, `"><`) {
		t.Fatalf("payload escaped badly: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt;&amp;") {
		t.Fatalf("unexpected escaping: %s", got)
	}
	if got := render(t, h.P("a\x00b")); got != "<p>a�b</p>" {
		t.Fatalf("NUL not replaced: %q", got)
	}
}

func TestURLSanitization(t *testing.T) {
	bad := []string{
		"javascript:alert(1)",
		"JAVASCRIPT:alert(1)",
		" javascript:alert(1)",
		"java\tscript:alert(1)",
		"java\nscript:alert(1)",
		"\x01javascript:alert(1)",
		"vbscript:msgbox(1)",
		"data:text/html,<script>alert(1)</script>",
		"data:image/svg+xml,<svg onload=alert(1)>",
	}
	for _, u := range bad {
		for _, n := range []*h.Element{h.A(h.Href(u)), h.Img(h.Src(u)), h.Form(h.Action(u)), h.Button(h.Attr("formaction", u)), h.A(h.Attr("xlink:href", u))} {
			if got := render(t, n); strings.Contains(strings.ToLower(got), "script:") || strings.Contains(got, "data:") {
				t.Errorf("%q not filtered: %s", u, got)
			}
		}
	}
	good := []string{"/path", "page?x=1:2", "#frag", "https://example.com/a?b=c&d", "mailto:a@b.c", "tel:+331", "//cdn.example.com/x.js", "./a:b"}
	for _, u := range good {
		if got := render(t, h.A(h.Href(u))); strings.Contains(got, h.FilteredValue) {
			t.Errorf("%q wrongly filtered: %s", u, got)
		}
	}
	if got := render(t, h.Img(h.Src("data:image/png;base64,AAAA"))); strings.Contains(got, h.FilteredValue) {
		t.Errorf("png data URL filtered in img src: %s", got)
	}
	if got := render(t, h.A(h.Href("data:image/png;base64,AAAA"))); !strings.Contains(got, h.FilteredValue) {
		t.Errorf("data URL allowed in href: %s", got)
	}
	got := render(t, h.Img(h.Attr("srcset", "/a.png 1x, javascript:alert(1) 2x")))
	if !strings.Contains(got, h.FilteredValue) {
		t.Errorf("srcset not filtered: %s", got)
	}
	if got := render(t, h.A().UnsafeURL("href", "javascript:void(0)")); !strings.Contains(got, "javascript:void(0)") {
		t.Errorf("UnsafeURL should bypass: %s", got)
	}
}

func TestScriptSinksRefused(t *testing.T) {
	for _, name := range []string{"onclick", "ONCLICK", "onmouseover", "x-data", "x-on:click", "@click", ":class", "v-on:click", "hx-on:click", "hx-on::after-request", "srcdoc", "_"} {
		err := renderErr(t, h.Div(h.Attr(name, "alert(1)")))
		if !h.IsSecurityError(err) {
			t.Errorf("%s: expected SecurityError, got %v", name, err)
		}
	}
	got := render(t, h.Button(h.On("click", "go()"), h.JSAttr("x-data", "{open: false}")))
	if got != `<button onclick="go()" x-data="{open: false}"></button>` {
		t.Fatal(got)
	}
	// Trusted handlers are still attribute-escaped: they cannot break out.
	got = render(t, h.Button(h.On("click", `"><img src=x onerror=alert(1)>`)))
	if strings.Contains(got, `"><img`) {
		t.Fatal(got)
	}
	if err := renderErr(t, h.Div().On("click()", "x")); !h.IsSecurityError(err) {
		t.Fatal(err)
	}
}

func TestInvalidNames(t *testing.T) {
	for _, name := range []string{"", "a b", `a"b`, "a=b", "a>b", "a/b", "a'b"} {
		if err := renderErr(t, h.Div(h.Attr(name, "v"))); !h.IsSecurityError(err) {
			t.Errorf("attr %q: %v", name, err)
		}
	}
	for _, tag := range []string{"", "1a", "a b", "a>", "img/src=x", "plaintext", "xmp"} {
		if err := renderErr(t, h.El(tag)); !h.IsSecurityError(err) {
			t.Errorf("tag %q: %v", tag, err)
		}
	}
	if got := render(t, h.El("my-widget", h.Attr("data-x.y", "1"))); got != `<my-widget data-x.y="1"></my-widget>` {
		t.Fatal(got)
	}
}

func TestScriptAndStyleContent(t *testing.T) {
	if err := renderErr(t, h.Script("alert(1)")); !h.IsSecurityError(err) {
		t.Fatal("plain text accepted in <script>")
	}
	if err := renderErr(t, h.StyleEl("body{}")); !h.IsSecurityError(err) {
		t.Fatal("plain text accepted in <style>")
	}
	for _, js := range []h.UnsafeJS{"x='</script><script>alert(1)'", "x='</SCRIPT >'", "<!--"} {
		if err := renderErr(t, h.Script(js)); !h.IsSecurityError(err) {
			t.Errorf("%q accepted", js)
		}
	}
	if err := renderErr(t, h.StyleEl(h.UnsafeCSS("</style><script>"))); !h.IsSecurityError(err) {
		t.Fatal("style breakout accepted")
	}
	got := render(t, h.Script(h.UnsafeJS("if (a < b && c) go()")))
	if got != `<script>if (a < b && c) go()</script>` {
		t.Fatal(got)
	}
	// Outside of <script>, UnsafeJS is just text.
	if got := render(t, h.Div(h.UnsafeJS("<b>"))); got != "<div>&lt;b&gt;</div>" {
		t.Fatal(got)
	}
}

func TestStyleSanitization(t *testing.T) {
	for _, css := range []string{
		"background:url(javascript:alert(1))",
		"width: expression(alert(1))",
		"-moz-binding:url(x.xml#x)",
		`color:red;\61 lert`,
		"x:</style>",
		"@import 'x'",
	} {
		if got := render(t, h.Div(h.Style(css))); strings.Contains(got, css) {
			t.Errorf("%q not filtered: %s", css, got)
		}
	}
	ok := "color: red; background: url(/img.png); width: calc(100% - 2px)"
	if got := render(t, h.Div(h.Style(ok))); !strings.Contains(got, ok) {
		t.Errorf("safe style filtered: %s", got)
	}
}

func TestTargetBlankGetsNoopener(t *testing.T) {
	if got := render(t, h.A(h.Href("/x"), h.Target("_blank"))); !strings.Contains(got, `rel="noopener noreferrer"`) {
		t.Fatal(got)
	}
	if got := render(t, h.A(h.Target("_blank"), h.Rel("external"))); !strings.Contains(got, `rel="external noopener"`) {
		t.Fatal(got)
	}
	if got := render(t, h.A(h.Target("_blank"), h.Rel("noreferrer"))); strings.Contains(got, "noopener") {
		t.Fatal(got)
	}
}

func TestMetaRefreshAndSVGAnimation(t *testing.T) {
	got := render(t, h.Meta(h.HTTPEquiv("refresh"), h.Content("0; url=javascript:alert(1)")))
	if strings.Contains(got, "javascript") {
		t.Fatal(got)
	}
	got = render(t, h.Meta(h.HTTPEquiv("refresh"), h.Content("5; url=/home")))
	if !strings.Contains(got, "url=/home") {
		t.Fatal(got)
	}
	got = render(t, h.El("animate", h.Attr("attributeName", "href"), h.Attr("values", "javascript:alert(1)")))
	if strings.Contains(got, "javascript") {
		t.Fatal(got)
	}
	if err := renderErr(t, h.El("set", h.Attr("attributeName", "onclick"), h.Attr("to", "alert(1)"))); !h.IsSecurityError(err) {
		t.Fatal(err)
	}
}

func TestComment(t *testing.T) {
	got := render(t, h.Comment("--><script>alert(1)</script><!--"))
	if strings.Count(got, "-->") != 1 || strings.Contains(got, "<script>") {
		t.Fatal(got)
	}
}

func TestFragmentPropsOutsideElement(t *testing.T) {
	if err := renderErr(t, h.Group(h.Class("x"))); !h.IsSecurityError(err) {
		t.Fatal(err)
	}
}

func TestRefusedElementLeavesNoPartialOutput(t *testing.T) {
	var buf bytes.Buffer
	err := h.Div("ok", h.A(h.Attr("onclick", "x"))).Render(&buf)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(buf.String(), "<a") {
		t.Fatalf("partial tag written: %s", buf.String())
	}
}

func TestWriteHTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := h.WriteHTTP(rec, 201, h.P("hi")); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 201 || rec.Body.String() != "<p>hi</p>" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("%d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	rec = httptest.NewRecorder()
	if err := h.WriteHTTP(rec, 200, h.Div(h.Attr("onclick", "x"))); err == nil || rec.Body.Len() != 0 {
		t.Fatalf("error page partially written: %v %q", err, rec.Body.String())
	}
}

type card struct {
	h.Builder[card]
	tone string
}

type tone string

func (c *card) AcceptChild(x any) bool {
	if t, ok := x.(tone); ok {
		c.tone = string(t)
		return true
	}
	return false
}

func newCard(children ...any) *card {
	c := &card{}
	c.Builder = h.NewBuilder(c, "section")
	return c.Children(children...)
}

func (c *card) Render(w io.Writer) error {
	return c.Elem().Clone().Data("tone", c.tone).Render(w)
}

func TestBuilderGeneric(t *testing.T) {
	c := newCard(tone("warm"), h.Class("p-2"), "body").Class("m-1").ID("c1").When(true, func(c *card) { c.Aria("label", "Card") })
	got := render(t, c)
	want := `<section class="p-2 m-1" id="c1" aria-label="Card" data-tone="warm">body</section>`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestSlot(t *testing.T) {
	slot := h.Button(h.Class("btn"), h.Data("slot", "button"), h.Style("color:red"), h.On("click", "a()"), h.Type("button"))
	child := h.A(h.Href("/login"), h.Class("underline"), h.Style("margin:0"), h.On("click", "b()"), "Login")
	out, err := h.Slot(slot, child, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := render(t, out)
	want := `<a class="btn underline" href="/login" style="color:red;margin:0" onclick="a();b()" data-slot="button" type="button">Login</a>`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if _, err := h.Slot(slot, h.Text("x"), nil); err == nil {
		t.Fatal("text child accepted")
	}
}

type valueProbe struct{}

func (valueProbe) Render(w io.Writer) error {
	v, ok := h.Lookup(w, "k")
	if !ok {
		v = "none"
	}
	_, err := io.WriteString(w, v.(string))
	return err
}

func TestWithValue(t *testing.T) {
	got := h.MustRenderString(h.Div(valueProbe{}, h.WithValue("k", "outer", valueProbe{}, h.Span(h.WithValue("k", "inner", valueProbe{})), valueProbe{}), valueProbe{}))
	if got != "<div>noneouter<span>inner</span>outernone</div>" {
		t.Fatal(got)
	}
}
