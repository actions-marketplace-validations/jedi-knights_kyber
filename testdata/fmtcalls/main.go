// Package fmtcalls is a kyber test fixture distinguishing pure fmt calls
// (Sprintf, Errorf, Sprint, Sprintln, Appendf, etc.) from I/O fmt calls
// (Println, Printf, Fprintln, ...). The testability metric should treat
// only the latter as side effects.
package fmtcalls

import "fmt"

// PureFmt uses only pure fmt — no observable I/O. Should NOT contribute to
// the side-effect count in testability.
func PureFmt(name string) (string, error) {
	msg := fmt.Sprintf("hello %s", name)
	if name == "" {
		return "", fmt.Errorf("empty name")
	}
	return msg, nil
}

// IOFmt writes to stdout — genuine I/O. Should contribute to the side-effect
// count.
func IOFmt(name string) {
	fmt.Println("hello", name)
	fmt.Printf("name: %s\n", name)
}
