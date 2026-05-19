// Package multi_return is a kyber test fixture for return-statement counting.
// Expected return count is 5 (hand-counted in the function body below).
package multi_return

// Classify has five return statements at various depths.
func Classify(x int) string {
	if x < 0 {
		return "negative"
	}
	if x == 0 {
		return "zero"
	}
	if x < 10 {
		return "small"
	}
	if x < 100 {
		return "medium"
	}
	return "large"
}
