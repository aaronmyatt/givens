package app

import "testing"

func TestRunBlocks(t *testing.T) {
	setup, err := ParseMarkdownSetup("1. Seed\n   ```bash run\n   ok seeded\n   ```\n" +
		"2. Look\n   ```js\n   1\n   ```\n3. Order\n   ```js run extra\n   2\n   ```\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		language string
		run      bool
	}{{"bash", true}, {"js", false}, {"js", true}}
	for index, step := range setup.Steps {
		block := step.Blocks[0]
		if block.Language != want[index].language || block.Run != want[index].run ||
			!block.Closed {
			t.Errorf("step %d's block is %+v", index+1, block)
		}
	}
	if setup.Steps[0].Blocks[0].Code != "ok seeded" {
		t.Errorf("the code is %q", setup.Steps[0].Blocks[0].Code)
	}
}
