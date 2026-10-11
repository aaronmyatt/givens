package boundedloops

func loops(n int, items []int) {
	for n > 0 { // want "loop has no stated bound"
		n--
	}
	for { // want "loop has no stated bound"
		break
	}
	// bound: n shrinks by one each pass and stops at zero
	for n > 0 {
		n--
	}
	for i := 0; i < n; i++ {
	}
	for range items {
	}
}
