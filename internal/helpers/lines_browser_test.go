package helpers

import "testing"

func TestUsesBrowser(t *testing.T) {
	cases := map[string]bool{
		"browser_open \"http://localhost:3000/cart\"\n":        true,
		"ORDER=\"$(in_browser 'window.app.lastOrderId()')\"\n": true,
		"  app_action signIn '{}'\n":                           true,
		"# browser_open is for later\nok done\n":               false,
		"ok \"no browser here\"\n":                             false,
		"my_browser_open_thing\n":                              false,
	}
	for text, want := range cases {
		if got := UsesBrowser(text); got != want {
			t.Errorf("UsesBrowser(%q) = %v, want %v", text, got, want)
		}
	}
}
