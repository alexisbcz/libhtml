# libhtml

Build HTML in Go with a JSX-like API. It escapes and sanitizes output by default.

## Install

```sh
go get github.com/alexisbcz/libhtml
```

## Use

```go
package main

import (
	"fmt"

	h "github.com/alexisbcz/libhtml"
)

func main() {
	page := h.Div(h.Class("card"),
		h.H1("Hello"),
		h.P("Any value can be a child: ", 42, " items."),
	)
	html, _ := h.RenderString(page)
	fmt.Println(html)
}
```

## License

MIT. See [LICENSE](LICENSE).
