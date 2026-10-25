package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chromedp/cdproto/target"

	"givens/internal/assert"
)

// Chrome's DevTools HTTP endpoints, on the port in DevToolsActivePort: /json/version names
// the browser's websocket, /json/list its tabs, and /json/new opens one.
// Ref: https://chromedevtools.github.io/devtools-protocol/#endpoints

// pagesMax bounds the tabs read from /json/list.
const pagesMax = 512

// listed is one target in /json/list.
type listed struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// browserURL is the websocket of profile's Chrome, or ErrNotRunning when none answers.
func browserURL(profile string) (string, error) {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(attachTimeout > 0, "the look has a limit")
	var version struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	if err := devtools(profile, http.MethodGet, "/json/version", &version); err != nil {
		return "", err
	}
	if !strings.HasPrefix(version.URL, "ws://") {
		return "", fmt.Errorf("%w: /json/version named no websocket", ErrNotRunning)
	}
	return version.URL, nil
}

// pageOf is the project's page: the one the page file names while Chrome still has it, or
// else its first tab, or else a new one. The page file then names it.
func pageOf(profile string) (target.ID, error) {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(pagesMax > 0, "tabs have a limit")
	saved := readSmall(filepath.Join(profile, pageFileName))
	tabs := make([]listed, 0, 8)
	if err := devtools(profile, http.MethodGet, "/json/list", &tabs); err != nil {
		return "", err
	}
	chosen := ""
	for _, tab := range tabs[:min(len(tabs), pagesMax)] {
		if tab.Type != "page" {
			continue
		}
		if tab.ID == saved {
			return targetID(saved), nil
		}
		if chosen == "" {
			chosen = tab.ID
		}
	}
	if chosen == "" {
		// The developer closed every window; Chrome on macOS keeps running without one.
		var opened listed
		if err := devtools(profile, http.MethodPut, "/json/new?about:blank", &opened); err != nil {
			return "", err
		}
		chosen = opened.ID
	}
	if chosen == "" || strings.ContainsAny(chosen, " \n") {
		return "", fmt.Errorf("%w: Chrome named no page", ErrNotRunning)
	}
	return targetID(chosen), writeSmall(filepath.Join(profile, pageFileName), chosen)
}

// devtools calls one of profile's Chrome's HTTP endpoints and decodes its JSON into into.
// Anything short of an answer is ErrNotRunning: a port file can outlive its Chrome.
func devtools(profile string, method string, path string, into any) error {
	assert.That(strings.HasPrefix(path, "/json/"), "only the DevTools endpoints are called")
	assert.That(into != nil, "the answer goes somewhere")
	lines := strings.SplitN(readSmall(filepath.Join(profile, portFileName)), "\n", 2)
	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port < 1 || port > 65535 {
		return ErrNotRunning
	}
	ctx, cancel := context.WithTimeout(context.Background(), attachTimeout)
	defer cancel()
	url := "http://127.0.0.1:" + strconv.Itoa(port) + path
	request, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		// Nothing listens: the port file outlived its Chrome. Said plainly, as the kit
		// shows it.
		return ErrNotRunning
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s answered %s", ErrNotRunning, path, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, ResultBytesMax))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotRunning, err)
	}
	return json.Unmarshal(body, into)
}

// Pages lists the ids of profile's Chrome's tabs, and Page says which is the project's: the
// steps check a Reset or a second run doesn't open another.
func Pages(profile string) ([]string, error) {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(pagesMax > 0, "tabs have a limit")
	tabs := make([]listed, 0, 8)
	if err := devtools(profile, http.MethodGet, "/json/list", &tabs); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(tabs))
	for _, tab := range tabs[:min(len(tabs), pagesMax)] {
		if tab.Type == "page" {
			ids = append(ids, tab.ID)
		}
	}
	return ids, nil
}

// Page is the project's page's id, as the page file names it, or "".
func Page(profile string) string {
	assert.That(filepath.IsAbs(profile), "the profile is absolute")
	assert.That(pageFileName != "", "the page file is named")
	return readSmall(filepath.Join(profile, pageFileName))
}
