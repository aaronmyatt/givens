package nestingdepth

func deep(a bool) {
	if a {
		for {
			switch {
			case a:
				if a {
					if a { // want "deep nests deeper than 4 levels"
					}
				}
			}
		}
	}
}

func chain(a, b, c, d, e bool) {
	if a {
	} else if b {
	} else if c {
	} else if d {
	} else if e {
	}
}
