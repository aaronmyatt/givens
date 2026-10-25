// Package shop is a fake web shop for the browser scenarios: one page, on every path, with
// the app actions an app's dev build would set on window.app. acceptance/world serves it in
// process; tools/fakeshop serves it for the Swift suites.
// See capabilities/mix-shell-browser-and-hand-steps.
package shop

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"givens/internal/assert"
)

// URL is the address the spec's browser scenarios name. Steps swap it for the fake shop's
// own, so nothing needs port 3000 free.
const URL = "http://localhost:3000"

// Page is the shop's page. signIn keeps who signed in, whoami says who, addToCart settles
// after the shop's delay and then tells the shop, lastOrderId says ord_42, and there's no
// refund.
const Page = `<!doctype html><title>Shop %[1]s</title><body>Shop %[1]s<script>
window.app = {
  signIn: (who) => { localStorage.setItem("user", who.email); return who.email },
  whoami: () => localStorage.getItem("user") || "",
  addToCart: (item) => new Promise(done => setTimeout(() => {
    fetch("/settled").then(() => done({cart: [item.sku]}))
  }, %[2]d)),
  lastOrderId: () => "ord_42",
};
</script></body>`

// Shop serves Page. GET /settled is the page saying addToCart settled; GET /was-settled
// answers "yes" or "no", for a suite in another process.
type Shop struct {
	AddToCartDelay time.Duration
	settled        atomic.Bool
}

// New is a shop whose addToCart settles after 2 seconds, as the spec says.
func New() *Shop {
	shop := &Shop{AddToCartDelay: 2 * time.Second}
	assert.That(shop.AddToCartDelay > 0, "addToCart takes a while")
	assert.That(!shop.settled.Load(), "nothing settled yet")
	return shop
}

// ServeHTTP answers one request. Ref: https://pkg.go.dev/net/http#Handler
func (s *Shop) ServeHTTP(out http.ResponseWriter, request *http.Request) {
	assert.That(s != nil, "the shop serves")
	assert.That(request != nil, "a request is answered")
	switch request.URL.Path {
	case "/settled":
		s.settled.Store(true)
	case "/was-settled":
		answer := "no"
		if s.Settled() {
			answer = "yes"
		}
		_, _ = fmt.Fprint(out, answer)
	default:
		_, _ = fmt.Fprintf(out, Page, strings.TrimPrefix(request.URL.Path, "/"),
			s.AddToCartDelay.Milliseconds())
	}
}

// Settled is true once a page's addToCart promise settled.
func (s *Shop) Settled() bool {
	assert.That(s != nil, "settled runs on a shop")
	assert.That(s.AddToCartDelay > 0, "the shop was made by New")
	return s.settled.Load()
}

// Has is true when Page sets window.app.<name>.
func Has(name string) bool {
	assert.That(name != "", "an action has a name")
	assert.That(!strings.Contains(name, " "), "an action's name is one word")
	return strings.Contains(Page, "  "+name+": ")
}
