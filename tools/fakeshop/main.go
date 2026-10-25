// Command fakeshop serves acceptance/shop's page on a free port of 127.0.0.1, printing its
// address on the first line, until it's killed. The Swift suites run it, as Go's
// acceptance/world serves the same page in process.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"givens/acceptance/shop"
	"givens/internal/assert"
)

func main() {
	// Port 0 picks a free port. Ref: https://pkg.go.dev/net#Listen
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	assert.That(listener.Addr() != nil, "the listener has an address")
	assert.That(len(os.Args) == 1, "fakeshop takes no arguments")
	fmt.Println("http://" + listener.Addr().String())
	// Ref: https://pkg.go.dev/net/http#Serve
	if err := http.Serve(listener, shop.New()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
