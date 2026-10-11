package runs

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/assert"
)

// authorTimeout bounds the one git call Author makes.
const authorTimeout = 5 * time.Second

// Author is git's user.name as seen from dir, or "" when git isn't installed, dir isn't in a
// repository with one, or git takes too long. A run is still recorded without a name.
// Ref: https://git-scm.com/docs/git-config#Documentation/git-config.txt-username
func Author(dir string) string {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	assert.That(authorTimeout > 0, "the git call has a limit")
	ctx, cancel := context.WithTimeout(context.Background(), authorTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "config", "user.name")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
