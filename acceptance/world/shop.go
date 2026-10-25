package world

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/acceptance/shop"
	"givens/internal/assert"
	"givens/internal/browser"
)

// ShopURL is the address the spec's browser scenarios name; see acceptance/shop.
const ShopURL = shop.URL

// Shop is the scenario's fake web shop, served in process.
type Shop struct {
	server *httptest.Server
	*shop.Shop
}

// ShopFor starts the scenario's fake shop the first time it's asked for.
// Ref: https://pkg.go.dev/net/http/httptest#NewServer
func (w *World) ShopFor() *Shop {
	assert.That(w != nil, "the shop belongs to a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	if w.Shop != nil {
		return w.Shop
	}
	fake := shop.New()
	w.Shop = &Shop{server: httptest.NewServer(fake), Shop: fake}
	return w.Shop
}

// URL is text with the fake shop's address in place of the spec's localhost:3000.
func (s *Shop) URL(text string) string {
	assert.That(s != nil && s.server != nil, "the shop is running")
	assert.That(!strings.Contains(text, "\n"), "one line is swapped")
	return strings.ReplaceAll(text, ShopURL, s.server.URL)
}

// Browser is the sample shop's Givens browser profile: in the scenario's folder, as setUp
// points GIVENS_BROWSERS there.
func (w *World) Browser() (string, error) {
	assert.That(w != nil, "browser runs on a world")
	assert.That(w.Project != "", "the sample shop was copied")
	return browser.Profile(w.Capabilities())
}

// closeBrowsers quits every Givens browser the scenario started and stops its shop. Chrome's
// helpers can still write in a profile for a moment after it quits, which tearDown's removal
// waits out.
func (w *World) closeBrowsers() error {
	assert.That(w != nil, "close runs on a world")
	assert.That(w.Dir != "", "the scenario has a folder")
	if w.Shop != nil {
		w.Shop.server.Close()
		w.Shop = nil
	}
	profiles, err := filepath.Glob(filepath.Join(w.Dir, "browsers", "*"))
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if err := browser.Close(profile); err != nil {
			return err
		}
	}
	return nil
}

// removeAll removes dir, trying again for a few seconds while Chrome's helpers finish.
func removeAll(dir string) error {
	assert.That(strings.Contains(dir, "givens-scenario-"), "only a scenario folder is removed")
	assert.That(filepath.IsAbs(dir), "the folder is absolute")
	var err error
	for range 50 {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}
