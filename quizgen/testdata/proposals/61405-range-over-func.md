# Proposal: range over func

We propose that a `for` statement with `range` accept a `func` value, so that
user-defined iterators compose with the `range` clause. An iterator is a
function that is passed a `yield` callback and calls it once per element.

A full iterator definition parses cleanly:

```go
func Count(n int) func(yield func(int) bool) {
    return func(yield func(int) bool) {
        for i := 0; i < n; i++ {
            if !yield(i) {
                return
            }
        }
    }
}
```

Consumers then range over the iterator directly. The fragment below is
deliberately incomplete — it has no surrounding package or function and a
missing brace — yet a lexer still tokenises it even though `go/parser` would
reject it:

```go
for value := range Count(10) {
    process(value)   // remaining lines elided ...
```

This difference between lexical scanning and full parsing is exactly why the
masker relies on `go/scanner` rather than `go/parser`.
