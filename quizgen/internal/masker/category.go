package masker

import "go/token"

// predeclared is Go's universe-scope identifiers (predeclared types,
// constants, and built-in functions — see go/types.Universe). These can
// never be ordinary sample-code variable or function names, so a distractor
// for one must come from this same set: offering "value" as a choice for an
// "int" blank is not a plausible wrong answer.
var predeclared = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true,
	"complex64": true, "complex128": true, "error": true,
	"float32": true, "float64": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"rune": true, "string": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"true": true, "false": true, "iota": true, "nil": true,
	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
}

// boilerplateKeywords are Go keywords that show up in almost every snippet
// regardless of what a proposal is actually about (declaration and
// control-flow scaffolding). They carry no proposal-specific meaning, so
// they are never masking targets in either code blocks or prose — unlike
// the remaining keywords (range, select, go, defer, chan, ...), which often
// *are* the language feature a proposal introduces.
var boilerplateKeywords = map[string]bool{
	"package": true, "import": true, "func": true, "var": true,
	"if": true, "else": true, "return": true, "for": true,
}

// category classifies a token name for distractor-pool matching: a
// predeclared identifier, a (non-boilerplate) keyword, or an ordinary
// identifier ("other"). GenerateChoices only draws distractors for a blank
// from candidates in the same category, since a predeclared type, a Go
// keyword, and an arbitrary sample-code identifier are never plausible
// substitutes for one another.
func category(name string) string {
	switch {
	case predeclared[name]:
		return "predeclared"
	case token.IsKeyword(name):
		return "keyword"
	default:
		return "other"
	}
}
