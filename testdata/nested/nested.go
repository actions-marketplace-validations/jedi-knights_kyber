// Package nested is a kyber test fixture exercising the nesting penalty of
// SonarSource Cognitive Complexity. Expected cognitive complexity is
// hand-computed below.
//
// Increments (each control structure adds 1 + nesting_level):
//   - outer for loop (nesting=0)               +1
//   - if inside the for (nesting=1)            +2
//   - inner for inside the if (nesting=2)      +3
//   - if inside the inner for (nesting=3)      +4
//
// Total: 1 + 2 + 3 + 4 = 10.
package nested

// Nested is intentionally deeply nested. Expected cognitive complexity: 10.
func Nested(xs []int) int {
	sum := 0
	for _, x := range xs {
		if x > 0 {
			for i := 0; i < x; i++ {
				if i%2 == 0 {
					sum += i
				}
			}
		}
	}
	return sum
}
