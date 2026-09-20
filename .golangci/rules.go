//go:build ignore

// Custom ruleguard rules, run by golangci-lint via gocritic (see .golangci.yml).
// The dsl import is satisfied by the go.mod dependency kept alive in tools.go.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

func capitalizedPanic(m dsl.Matcher) {
	m.Match(
		`panic($s)`,
		`panic(fmt.Sprintf($s, $*_))`,
		`panic(fmt.Errorf($s, $*_))`,
		`panic(errors.New($s))`,
	).
		Where(m["s"].Text.Matches(`^"[A-Z]`)).
		Report("panic message should not be capitalized")
}

// Flags prose-style capitalized test failures ("Expected: ...") but allows the idiomatic
// `Func(args) = got, want ...` form, whose first word is a capitalized identifier followed by "(".
func capitalizedTestFailure(m dsl.Matcher) {
	m.Match(
		`$t.Error($s, $*_)`,
		`$t.Errorf($s, $*_)`,
		`$t.Fatal($s, $*_)`,
		`$t.Fatalf($s, $*_)`,
	).
		Where((m["t"].Type.Is(`*testing.T`) || m["t"].Type.Is(`*testing.B`)) &&
			m["s"].Text.Matches(`^"[A-Z][A-Za-z]*[ :,.]`)).
		Report("test failure message should be lowercase, in the form `Func(args) = got, want <expected>`")
}
