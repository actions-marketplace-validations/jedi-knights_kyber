// Package npath_branchy is a kyber test fixture for NPath complexity.
// Expected NPath is hand-computed below.
//
// Sequential statements multiply paths; an if-else block contributes
// P(if) + P(else); an if without else contributes P(if) + 1.
//
//   - Three sequential if-else blocks: each contributes 2 paths.
//   - Sequential composition multiplies: 2 × 2 × 2 = 8.
//
// Total NPath complexity: 8.
package npath_branchy

// Triple contains three sequential if-else blocks. Expected NPath: 8.
func Triple(a, b, c bool) int {
	x := 0
	if a {
		x++
	} else {
		x--
	}
	if b {
		x++
	} else {
		x--
	}
	if c {
		x++
	} else {
		x--
	}
	return x
}
