package setup

import (
	"os"
	"path/filepath"
	"strings"

	"givens/internal/assert"
)

// A setup card's "bash run" or "js run" block runs as a script of Givens's own: the kit,
// then the block's code, or in_browser of it for JavaScript. Drop in and givens run both
// write it here, so a block acts the same in the app and in a terminal.
// See capabilities/mix-shell-browser-and-hand-steps.

// ScriptAsEnv names the setup a run block stands for, so the kit loads that capability's
// own helpers, as for its .sh. See GIVENS_SCRIPT in internal/kit/givens.sh.
const ScriptAsEnv = "GIVENS_SCRIPT_AS"

// BlockLanguage says how a fenced block runs: "bash", "js", or "" when it doesn't.
func BlockLanguage(language string, run bool, closed bool) string {
	assert.That(len(language) < 64, "a language is one word")
	assert.That(!strings.ContainsAny(language, " \n"), "a language is one word")
	if !run || !closed {
		return ""
	}
	switch language {
	case "bash", "sh":
		return "bash"
	case "js", "javascript":
		return "js"
	}
	return ""
}

// WriteBlock writes the script a run block runs as into dir, named after name, and returns
// its path. kit is the capabilities folder's givens.sh; language is BlockLanguage's.
func WriteBlock(dir string, name string, kit string, language string, code string) (string,
	error) {
	assert.That(filepath.IsAbs(dir) && filepath.IsAbs(kit), "the folders are absolute")
	assert.That(language == "bash" || language == "js", "only a run block is written")
	base := filepath.Join(dir, name)
	body := "source " + Quote(kit) + "\n" + code + "\n"
	if language == "js" {
		if err := os.WriteFile(base+".js", []byte(code), 0o644); err != nil {
			return "", err
		}
		body = "source " + Quote(kit) + "\nin_browser \"$(cat " + Quote(base+".js") + ")\"\n"
	}
	return base + ".sh", os.WriteFile(base+".sh", []byte(body), 0o644)
}

// Quote quotes text as one bash word.
// Ref: https://www.gnu.org/software/bash/manual/html_node/Single-Quotes.html
func Quote(text string) string {
	assert.That(!strings.ContainsRune(text, 0), "a word has no NUL")
	quoted := "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
	assert.That(len(quoted) >= len(text)+2, "the quotes are added")
	return quoted
}
