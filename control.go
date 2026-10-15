package libhtml

import (
	"cmp"
	"io"
	"iter"
	"maps"
	"slices"
)

// The control-flow helpers are generic, so they work for anything that can be
// a child: nodes, props, strings or whole fragments.
//
//	h.Div(
//		h.If(active, h.Class("active")),
//		h.IfElse(user != nil, h.Text("Hi"), h.Text("Sign in")),
//		h.Iff(user != nil, func() h.Node { return h.Text(user.Name) }),
//	)
//
// If and IfElse evaluate their arguments eagerly, like any Go call. When a
// branch would dereference something that may be nil, use the F variants
// (Iff, IfElseF), which only build the branch that is taken.

// If returns v when cond is true and the zero value (rendered as nothing)
// otherwise.
func If[T any](cond bool, v T) T {
	if cond {
		return v
	}
	var zero T
	return zero
}

// Unless returns v when cond is false.
func Unless[T any](cond bool, v T) T { return If(!cond, v) }

// Iff calls fn only when cond is true.
func Iff[T any](cond bool, fn func() T) T {
	if cond {
		return fn()
	}
	var zero T
	return zero
}

// IfElse returns a when cond is true and b otherwise.
func IfElse[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// IfElseF calls fa when cond is true and fb otherwise.
func IfElseF[T any](cond bool, fa, fb func() T) T {
	if cond {
		return fa()
	}
	return fb()
}

// Ternary is IfElse under the name most languages use; handy for values such
// as class names: h.Class(h.Ternary(open, "block", "hidden")).
func Ternary[T any](cond bool, a, b T) T { return IfElse(cond, a, b) }

// Map renders fn(item) for each item.
func Map[T, R any](items []T, fn func(T) R) Fragment {
	out := make(Fragment, len(items))
	for i, it := range items {
		out[i] = fn(it)
	}
	return out
}

// MapIdx is Map with the item's index.
func MapIdx[T, R any](items []T, fn func(int, T) R) Fragment {
	out := make(Fragment, len(items))
	for i, it := range items {
		out[i] = fn(i, it)
	}
	return out
}

// MapKV renders fn(k, v) for each map entry, in sorted key order so output is
// deterministic.
func MapKV[K cmp.Ordered, V, R any](m map[K]V, fn func(K, V) R) Fragment {
	keys := slices.Sorted(maps.Keys(m))
	out := make(Fragment, len(keys))
	for i, k := range keys {
		out[i] = fn(k, m[k])
	}
	return out
}

// MapSeq renders fn(item) for each item of an iterator.
func MapSeq[T, R any](seq iter.Seq[T], fn func(T) R) Fragment {
	var out Fragment
	for it := range seq {
		out = append(out, fn(it))
	}
	return out
}

// Range renders fn(i) for i in [0, n).
func Range[R any](n int, fn func(int) R) Fragment {
	out := make(Fragment, max(n, 0))
	for i := range out {
		out[i] = fn(i)
	}
	return out
}

// Filter returns the items for which keep returns true. Combine with Map.
func Filter[T any](items []T, keep func(T) bool) []T {
	var out []T
	for _, it := range items {
		if keep(it) {
			out = append(out, it)
		}
	}
	return out
}

// Switch starts a switch on v:
//
//	h.Switch(status).
//		Case("ok", h.Text("All good")).
//		Case("warn", h.Text("Careful")).
//		Default(h.Text("Unknown"))
//
// The result is a Node; the first matching case is rendered.
func Switch[T comparable](v T) *SwitchBuilder[T] { return &SwitchBuilder[T]{value: v} }

// SwitchBuilder is returned by Switch.
type SwitchBuilder[T comparable] struct {
	value   T
	matched bool
	result  any
}

// Case selects child when the switch value equals v and no earlier case
// matched.
func (s *SwitchBuilder[T]) Case(v T, child any) *SwitchBuilder[T] {
	if !s.matched && s.value == v {
		s.matched, s.result = true, child
	}
	return s
}

// CaseF is Case with a lazily built child.
func (s *SwitchBuilder[T]) CaseF(v T, fn func() any) *SwitchBuilder[T] {
	if !s.matched && s.value == v {
		s.matched, s.result = true, fn()
	}
	return s
}

// Default selects child when no case matched.
func (s *SwitchBuilder[T]) Default(child any) *SwitchBuilder[T] {
	if !s.matched {
		s.matched, s.result = true, child
	}
	return s
}

// Render renders the selected child.
func (s *SwitchBuilder[T]) Render(w io.Writer) error {
	return Fragment{s.result}.Render(w)
}

func (s *SwitchBuilder[T]) renderTo(r *renderer) { Fragment{s.result}.renderTo(r) }

// Fragment returns the selected child as a fragment, so props selected by a
// switch are applied to the enclosing element.
func (s *SwitchBuilder[T]) Fragment() Fragment { return Fragment{s.result} }
