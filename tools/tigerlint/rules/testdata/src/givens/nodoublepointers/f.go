package nodoublepointers

func one(p *int) int { return *p }

func two(p **int) int { // want "two levels of pointer"
	return **p // want "two levels of pointer"
}
