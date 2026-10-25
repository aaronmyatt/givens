package browser

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// shopPage is a page with app actions, as an app's dev build sets them.
const shopPage = `<!doctype html><title>Shop</title><script>
window.app = {
  signIn: (who) => { document.title = "Signed in as " + who.email; return who.email },
  addToCart: (item) => new Promise(done => setTimeout(() => done({cart: [item.sku]}), 200)),
  lastOrderId: () => "ord_42",
};
</script>`

// headless starts a page server and gives a profile of the test's own, headless, whose
// Chrome is closed when the test ends. Chrome's helpers can still write in the profile for a
// moment after it quits, so it's removed with a few tries rather than by t.TempDir.
func headless(t *testing.T) (string, string) {
	t.Helper()
	if _, ok := Chrome(); !ok {
		t.Fatal("these tests need Chrome: brew install --cask google-chrome")
	}
	t.Setenv(HeadlessEnv, "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(shopPage))
	}))
	t.Cleanup(server.Close)
	folder, err := os.MkdirTemp("", "givens-browser-test")
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(folder, "profile")
	t.Cleanup(func() {
		if err := Close(profile); err != nil {
			t.Errorf("closing Chrome: %v", err)
		}
		for range 50 {
			if os.RemoveAll(folder) == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	return profile, server.URL
}

func TestOpenThenEvaluate(t *testing.T) {
	profile, shop := headless(t)
	if Running(profile) {
		t.Fatal("a new profile has no Chrome running")
	}
	if err := Open(profile, shop+"/cart"); err != nil {
		t.Fatal(err)
	}
	if !Running(profile) {
		t.Fatal("Open left no Chrome running")
	}
	cases := map[string]string{
		`location.pathname`:                  "/cart",
		`({a: 1, b: [2, 3]})`:                `{"a":1,"b":[2,3]}`,
		`undefined`:                          "",
		`window.app.addToCart({sku: "tee"})`: `{"cart":["tee"]}`,
	}
	for js, want := range cases {
		got, err := Eval(profile, js)
		if err != nil || got != want {
			t.Errorf("Eval(%s) = %q, %v; want %q", js, got, err, want)
		}
	}
}

func TestOneBrowserOnePage(t *testing.T) {
	profile, shop := headless(t)
	if err := Open(profile, shop+"/cart"); err != nil {
		t.Fatal(err)
	}
	first := readSmall(filepath.Join(profile, pageFileName))
	if _, err := Eval(profile, `window.marked = "kept"`); err != nil {
		t.Fatal(err)
	}
	if err := Open(profile, shop+"/admin"); err != nil {
		t.Fatal(err)
	}
	if again := readSmall(filepath.Join(profile, pageFileName)); again != first || first == "" {
		t.Fatalf("the page moved from %q to %q", first, again)
	}
	if got, _ := Eval(profile, `location.pathname`); got != "/admin" {
		t.Fatalf("the page is at %q, not /admin", got)
	}
}

func TestErrors(t *testing.T) {
	profile, shop := headless(t)
	if _, err := Eval(profile, `1`); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Eval with no Chrome = %v, want ErrNotRunning", err)
	}
	if err := Open(profile, shop); err != nil {
		t.Fatal(err)
	}
	_, err := Eval(profile, `window.app.nope()`)
	if err == nil || err.Error() != "window.app.nope is not a function" {
		t.Errorf("a throw says %v", err)
	}
	_, err = Action(profile, "refund", `{"order":"ord_42"}`)
	if !errors.Is(err, ErrNoAction) || !strings.Contains(err.Error(), "window.app.refund") {
		t.Errorf("a missing action says %v", err)
	}
	if got, err := Action(profile, "signIn", `{"email":"sam@example.com"}`); err != nil ||
		got != "sam@example.com" {
		t.Errorf("signIn = %q, %v", got, err)
	}
	if _, err := Action(profile, "a.b", ""); err == nil {
		t.Error("a name with a dot was taken")
	}
	if _, err := evaluate(profile, `new Promise(() => {})`, time.Second); !errors.Is(err,
		ErrNoAnswer) {
		t.Errorf("a promise that never settles gives %v", err)
	}
}

func TestProfileFollowsSymlinks(t *testing.T) {
	folder := t.TempDir()
	t.Setenv(DirEnv, filepath.Join(folder, "browsers"))
	real := filepath.Join(folder, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(folder, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	a, errA := Profile(real)
	b, errB := Profile(link)
	if errA != nil || errB != nil || a != b {
		t.Fatalf("Profile(real) = %q, Profile(link) = %q", a, b)
	}
}
