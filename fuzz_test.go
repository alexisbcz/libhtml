package libhtml_test

import (
	"strings"
	"testing"

	h "github.com/alexisbcz/libhtml"
)

// FuzzNoInjection checks that arbitrary input in text, attribute and URL
// positions can never add markup or leave its attribute: the output always
// has exactly the structure the code built.
func FuzzNoInjection(f *testing.F) {
	for _, s := range []string{"", "plain", `"><script>alert(1)</script>`, "javascript:alert(1)", "java\tscript:x", "' onmouseover='x", "</div><!--", "\x00", "&amp;"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, err := h.RenderString(h.Div(h.TitleAttr(s), h.Data("x", s), h.Class(s), s, h.A(h.Href(s), s), h.Comment(s)))
		if err != nil {
			t.Fatal(err)
		}
		// div, a and the comment each contribute fixed '<' and '>' counts.
		if n := strings.Count(out, "<"); n != 5 {
			t.Fatalf("%d '<' in %q", n, out)
		}
		// Outside the comment (where quotes are inert), only the four
		// attributes' delimiters may appear; class is omitted when blank.
		i, j := strings.Index(out, "<!--"), strings.LastIndex(out, "-->")
		markup := out[:i] + out[j:]
		want := 8
		if strings.Trim(s, " \t\n\r\f") == "" { // HTML whitespace, not unicode.IsSpace
			want = 6
		}
		if n := strings.Count(markup, `"`); n != want {
			t.Fatalf("%d quotes in %q", n, out)
		}
		href := out[strings.Index(out, `href="`)+6:]
		href = strings.ToLower(href[:strings.IndexByte(href, '"')])
		var b strings.Builder
		for _, c := range []byte(href) {
			if c > 0x20 && c != 0x7f {
				b.WriteByte(c)
			}
		}
		if n := b.String(); strings.HasPrefix(n, "javascript:") || strings.HasPrefix(n, "vbscript:") || strings.HasPrefix(n, "data:") {
			t.Fatalf("dangerous href %q", href)
		}
	})
}
