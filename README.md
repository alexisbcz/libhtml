# libhtml

[Apache-2.0 license](./LICENSE)

[![Join Discord](https://img.shields.io/badge/Join%20Discord-gray?style=flat&logo=discord&logoColor=white&link=https://discord.gg/eMUC7ejHja)](https://discord.gg/eMUC7ejHja)

> A simple library to write HTML in plain Go.

## Installation

```bash
go get github.com/alexisbcz/libhtml
```

## Example

```go
func SignUpForm(errors validator.Errors) html.Node {
	return html.Form(
		html.Div(
			ui.Field(
				ui.Label(ui.LabelProps{Text: "First Name", For: "firstName"})(),
				ui.Input(ui.InputProps{Type: "firstName", Id: "firstName", Placeholder: "John"})(),
				ui.Errors("firstName", errors),
			),
			ui.Field(
				ui.Label(ui.LabelProps{Text: "Last Name", For: "lastName"})(),
				ui.Input(ui.InputProps{Type: "lastName", Id: "lastName", Placeholder: "Doe"})(),
				ui.Errors("lastName", errors),
			),
		).Class("grid sm:grid-cols-2 gap-4"),
		ui.Field(
			ui.Label(ui.LabelProps{Text: "Email address", For: "email"})(),
			ui.Input(ui.InputProps{Type: "email", Id: "email", Placeholder: "john.doe@example.com"})(),
			ui.Errors("email", errors),
		),
		ui.Field(
			ui.Label(ui.LabelProps{Text: "Password", For: "password"})(),
			ui.Input(ui.InputProps{Type: "password", Id: "password", Placeholder: "················"})(),
			ui.Errors("password", errors),
		),
		ui.Button(ui.ButtonProps{Text: "Sign Up", Type: "submit"}),
		html.P(
			html.Text("Already have an account?"),
			html.A(html.Text("Sign in")).Href("/sign-in").
				Class("ml-1 cursor-pointer underline text-blue-700 hover:text-blue-600 transition-colors"),
		).Class("text-sm text-neutral-800 text-center"),
	).
		Id("sign-up-form").
		Attribute("hx-post", "/sign-up/").
		Attribute("hx-swap", "outerHTML").
		Attribute("hx-target", "#sign-up-form").
		Class("flex flex-col gap-y-5")
}
```

## Usage

*yet to be written*

## Acknowledgements

Thanks to the awesome work from [gostar](https://github.com/delaneyj/gostar), [htmlgo](https://github.com/maddalax/htmgo) and [gomponents](https://github.com/maragudk/gomponents) which inspired me to create yet another `HTML in plain Go` library.