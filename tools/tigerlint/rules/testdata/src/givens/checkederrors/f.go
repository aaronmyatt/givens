package checkederrors

import (
	"fmt"
	"os"
	"strings"
)

func calls() {
	os.Remove("x") // want "error dropped"
	_ = os.Remove("x")
	fmt.Println("printing to stdout is exempt")
	var b strings.Builder
	b.WriteString("an in-memory builder is exempt")
	if err := os.Remove("x"); err != nil {
		return
	}
}
