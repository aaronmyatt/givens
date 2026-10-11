package sizedmake

func make1() {
	_ = make(map[string]int) // want "make without a size"
	_ = make(map[string]int, 4)
	_ = make([]int, 0, 4)
	_ = make(chan int)
}
