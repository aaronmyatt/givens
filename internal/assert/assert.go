// Package assert states what a function expects, and crashes the program when that is false.
//
// Tiger Style: "Assertions detect programmer errors. Unlike operating errors, which are expected
// and which must be handled, assertion failures are unexpected. The only correct way to handle
// corrupt code is to crash." Operating errors (a missing file, a failed command) are returned as
// error values instead.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
//
// Power of Ten, rule 5: "The assertion density of the code should average to a minimum of two
// assertions per function." tools/tigerlint counts calls into this package to enforce it.
// Ref: https://en.wikipedia.org/wiki/The_Power_of_10:_Rules_for_Developing_Safety-Critical_Code
package assert

// That crashes the program with message when ok is false.
// Assert both the positive space (what must hold) and the negative space (what must not).
func That(ok bool, message string) {
	if !ok {
		// panic is allowed only here: tigerlint's simpleflow rule bans it everywhere else.
		// Ref: https://go.dev/ref/spec#Handling_panics
		panic("assertion failed: " + message)
	}
}

// Unreachable crashes the program: call it where control flow must never arrive,
// such as the default branch of a switch that covers every case.
func Unreachable(message string) {
	panic("unreachable: " + message)
}
