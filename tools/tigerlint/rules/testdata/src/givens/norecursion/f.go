package norecursion

func countdown(n int) int {
	if n == 0 {
		return 0
	}
	return countdown(n - 1) // want "countdown calls itself"
}

type tree struct{ left *tree }

func (t *tree) depth() int {
	if t == nil {
		return 0
	}
	return 1 + t.left.depth() // want "depth calls itself"
}

func helper() int { return countdown(3) }
