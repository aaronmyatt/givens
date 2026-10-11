package assertdensity

import "givens/internal/assert"

func checked(n int) int { // want "package assertdensity has 3 asserts across 2 functions"
	assert.That(n >= 0, "n is not negative")
	assert.That(n < 10, "n is small")
	return n
}

func unchecked(n int) int {
	assert.That(n > 0, "only one assert here")
	return n
}
