/**
 * Copyright 2025 Alexis Bouchez <alexbcz@proton.me>
 *
 * This file is part of lib
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

package html_test

import (
	"strings"
	"testing"

	. "github.com/alexisbcz/libhtml"
)

func TestRender(t *testing.T) {
	type Profile struct {
		FirstName string
		LastName  string
	}

	type User struct {
		ID      int
		Profile *Profile
	}

	users := []User{
		{ID: 1},
		{ID: 2, Profile: &Profile{FirstName: "Alexis", LastName: "Bouchez"}},
	}

	activateDarkMode := true

	doc := Document().Children(
		HTML().ClassIf(activateDarkMode, "dark").Lang("en").Children(
			Head().Children(
				Title().Children(Text("My Go application")),
				Meta().Charset("utf-8"),
				Meta().Charset("width=device-width, initial-scale=1"),
			),
			Body().Children(
				Div().Children(
					Map(users, func(user User) Node {
						return IfElseFunc(user.Profile != nil, func() Node {
							return P(Textf("Hello %s %s", user.Profile.FirstName, user.Profile.LastName))
						}, func() Node {
							return P(Text("Hello anonymous"))
						})
					}),
				),
			),
		),
	)

	const expected = `<!DOCTYPE html><html class="dark" lang="en"><head><title>My Go application</title><meta charset="utf-8"/><meta charset="width=device-width, initial-scale=1"/></head><body><div><p>Hello anonymous</p><p>Hello Alexis Bouchez</p></div></body></html>`

	sb := &strings.Builder{}

	if err := doc.Render(sb); err != nil {
		t.Error(err)
	}

	got := sb.String()
	if expected != got {
		t.Errorf("expected: \"%s\"; got: \"%s\"", expected, got)
	}
}
