// Package browser is the Givens browser: one Chrome per project, with a profile of Givens's
// own, that setup drives over the Chrome DevTools Protocol and the developer carries on in.
// The givens command and the app both reach it through this package, so a terminal run and a
// Drop in share one browser. See capabilities/mix-shell-browser-and-hand-steps.
//
// Chrome is started with chromedp's KeepOpen, so it outlives the givens command that started
// it, and found again through its profile's DevToolsActivePort file and the DevTools HTTP
// endpoints. Each call attaches to the project's one page, runs, and detaches.
// Ref: https://chromedevtools.github.io/devtools-protocol/
// Ref: https://pkg.go.dev/github.com/chromedp/chromedp@v0.21.0
package browser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"

	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
const (
	// OpenSecondsMax bounds browser_open: starting Chrome, then loading the page.
	OpenSecondsMax = 30
	// EvalSecondsMax bounds in_browser and app_action, promise included.
	EvalSecondsMax = 30
	// ResultBytesMax bounds what one evaluation returns, as JSON.
	ResultBytesMax = 1 << 20
	// startTimeout bounds Chrome's start; a new profile took 6 s on an M-series Mac.
	startTimeout = 20 * time.Second
	// attachTimeout bounds the DevTools HTTP calls and attaching to the page.
	attachTimeout = 5 * time.Second
	// closeTimeout bounds asking Chrome to quit.
	closeTimeout = 5 * time.Second
)

// Environment variables a developer, or a test, sets.
const (
	// DirEnv moves the folder holding every project's profile. The acceptance suites set it
	// to the scenario's folder.
	DirEnv = "GIVENS_BROWSERS"
	// ChromeEnv names another Chromium-based browser: Chromium, Brave, Edge.
	ChromeEnv = "GIVENS_CHROME"
	// HeadlessEnv=1 runs Chrome without a window, for the test suites.
	HeadlessEnv = "GIVENS_HEADLESS"
)

// Files in a profile beside Chrome's own.
const (
	// portFileName is where Chrome writes the port it picked for --remote-debugging-port=0.
	// Ref: https://chromedevtools.github.io/devtools-protocol/#how-do-i-access-the-browser-target
	portFileName = "DevToolsActivePort"
	// pageFileName holds the target id of the project's page, so every call uses one tab.
	pageFileName = "givens-page"
	// smallFileBytesMax bounds the port and page files.
	smallFileBytesMax = 4096
)

// Errors a caller tells apart.
var (
	ErrNoChrome   = errors.New("this scenario needs Chrome")
	ErrNotRunning = errors.New("the Givens browser isn't open")
	ErrNoAnswer   = fmt.Errorf("no answer after %ds", EvalSecondsMax)
	ErrNoAction   = errors.New("the page has no such app action")
)

// Profile is the folder of capabilities' browser: <user cache>/Givens/browsers/<hash>, or under
// GIVENS_BROWSERS. The path is hashed after symlinks are resolved, so the app and a terminal
// find the same profile from /var and /private/var alike.
// Ref: https://pkg.go.dev/os#UserCacheDir
func Profile(capabilities string) (string, error) {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(len(capabilities) < 4096, "the path is a path")
	resolved, err := filepath.EvalSymlinks(capabilities)
	if err != nil {
		return "", err
	}
	root := os.Getenv(DirEnv)
	if root == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(cache, "Givens", "browsers")
	}
	sum := sha256.Sum256([]byte(resolved))
	return filepath.Join(root, hex.EncodeToString(sum[:])[:16]), nil
}

// Chrome is the browser to start: GIVENS_CHROME, or Chrome or Chromium where macOS keeps
// them. It says false when there's none.
func Chrome() (string, bool) {
	paths := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	if named := os.Getenv(ChromeEnv); named != "" {
		paths = []string{named}
	}
	assert.That(len(paths) > 0, "somewhere is looked")
	assert.That(len(paths) <= 2, "only a few places are looked")
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// Running is true when profile's Chrome answers.
func Running(profile string) bool {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(attachTimeout > 0, "the look has a limit")
	_, err := browserURL(profile)
	return err == nil
}

// Open loads url in the project's page, starting Chrome first when it isn't running, and
// waits for the page to load. Needed by browser_open.
func Open(profile string, url string) error {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(url != "", "a page is named")
	if !Running(profile) {
		if err := start(profile); err != nil {
			return err
		}
	}
	navigate := chromedp.Steps(chromedp.Navigate(url), bringToFront())
	_, err := onPage(profile, navigate, OpenSecondsMax*time.Second)
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s didn't load within %ds", url, OpenSecondsMax)
	}
	return err
}

// bringToFront shows the project's page, as the developer carries on in it.
// Ref: https://chromedevtools.github.io/devtools-protocol/tot/Page/#method-bringToFront
func bringToFront() chromedp.Action[chromedp.Void] {
	assert.That(page.BringToFront.Method != "", "the command is named")
	assert.That(OpenSecondsMax > 0, "bringing to front is part of an open")
	return func(ctx context.Context, tab *chromedp.Target) (chromedp.Void, error) {
		_, err := cdp.Call(ctx, tab, page.BringToFront, cdp.Empty{})
		return chromedp.Void{}, err
	}
}

// start launches Chrome on profile with chromedp's KeepOpen: detached, with
// --remote-debugging-port=0, and left running when this process ends.
// Ref: https://pkg.go.dev/github.com/chromedp/chromedp@v0.21.0#KeepOpen
func start(profile string) error {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(startTimeout > attachTimeout, "a start has longer than an attach")
	chrome, ok := Chrome()
	if !ok {
		return ErrNoChrome
	}
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return err
	}
	// A port file left by a Chrome that quit would name a dead port.
	if err := os.Remove(filepath.Join(profile, portFileName)); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		return err
	}
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome),
		chromedp.UserDataDir(profile), chromedp.KeepOpen, remote.WebSocket)
	if os.Getenv(HeadlessEnv) != "1" {
		options = append(options, chromedp.VisibleWindow)
	}
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	allocator, cancelAllocator := chromedp.NewExecAllocator(ctx, options...)
	defer cancelAllocator()
	tab, cancelTab := chromedp.NewContext(allocator)
	defer cancelTab()
	if err := chromedp.Do(tab); err != nil {
		return fmt.Errorf("Chrome didn't start: %w", err)
	}
	id := chromedp.FromContext(tab).Target.TargetID
	return writeSmall(filepath.Join(profile, pageFileName), string(id))
}

// onPage attaches to the project's page and runs action there within limit. It detaches
// after, leaving the page and Chrome open.
func onPage[T any](profile string, action chromedp.Action[T], limit time.Duration) (T, error) {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(limit > 0, "a call has a limit")
	var zero T
	url, err := browserURL(profile)
	if err != nil {
		return zero, err
	}
	id, err := pageOf(profile)
	if err != nil {
		return zero, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	// NoModifyURL: the URL already names the browser's websocket.
	allocator, cancelAllocator := remote.NewAllocator(ctx, url, remote.NoModifyURL)
	defer cancelAllocator()
	// Detach on cancel: by default, a context closes the tab WithTargetID attached it to.
	tab, cancelTab := chromedp.NewContext(allocator, chromedp.WithTargetID(id),
		chromedp.WithDetachOnCancel())
	defer cancelTab()
	return chromedp.Run(tab, action)
}

// Close asks profile's Chrome to quit, if it's running. The suites close each scenario's.
// chromedp keeps Browser.close to itself, so it's sent on a connection of its own.
// Ref: https://chromedevtools.github.io/devtools-protocol/tot/Browser/#method-close
func Close(profile string) error {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(closeTimeout > 0, "a close has a limit")
	url, err := browserURL(profile)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	conn, err := remote.DialContext(ctx, url)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	quit := &cdproto.Message{ID: 1, Method: cdproto.MethodType(cdpbrowser.CommandClose)}
	if err := conn.Write(ctx, quit); err != nil {
		return err
	}
	// bound: closeTimeout ends the wait.
	for Running(profile) {
		if ctx.Err() != nil {
			return fmt.Errorf("Chrome on %s didn't quit within %s", profile, closeTimeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// readSmall reads a short file, trimmed, or "" when it's missing or too big.
func readSmall(path string) string {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	assert.That(smallFileBytesMax > 0, "small files have a limit")
	info, err := os.Stat(path)
	if err != nil || info.Size() > smallFileBytesMax {
		return ""
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(text))
}

// writeSmall writes a one-line file.
func writeSmall(path string, text string) error {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	assert.That(len(text) < smallFileBytesMax, "a small file stays small")
	return os.WriteFile(path, []byte(text+"\n"), 0o644)
}

// targetID is a page's id, as the page file and /json/list give it.
func targetID(text string) target.ID {
	assert.That(!strings.ContainsAny(text, " \n"), "an id is one word")
	assert.That(len(text) < smallFileBytesMax, "an id is short")
	return target.ID(text)
}
