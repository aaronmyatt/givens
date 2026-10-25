package app

import "testing"

const payInApp = "You'll need the TestFlight build of the shop app.\n\n" +
	"1. Install it on your phone: [Shop beta](https://testflight.apple.com/join/abc)\n" +
	"2. Sign in as `sam@example.com`\n\n" +
	"   ```bash\n   sudo -u shop shopctl flags set new_checkout on\n     indented\n   ```\n" +
	"3. Add the test card `4242 4242 4242 4242` under Wallet\n\n" +
	"   ![Wallet screen](../images/wallet.png)\n\n" +
	"   ```js\n   localStorage.removeItem(\"cart\")\n"

func TestParseMarkdownSetup(t *testing.T) {
	setup, err := ParseMarkdownSetup(payInApp)
	if err != nil {
		t.Fatal(err)
	}
	if setup.Intro != "You'll need the TestFlight build of the shop app." || len(setup.Steps) != 3 {
		t.Fatalf("setup = %+v", setup)
	}
	first, second, third := setup.Steps[0], setup.Steps[1], setup.Steps[2]
	if len(first.Links) != 1 || first.Links[0] != "https://testflight.apple.com/join/abc" {
		t.Errorf("step 1 = %+v", first)
	}
	if len(second.Blocks) != 1 || second.Blocks[0].Language != "bash" ||
		second.Blocks[0].Code != "sudo -u shop shopctl flags set new_checkout on\n  indented" ||
		!second.Blocks[0].Closed || second.Inline[0] != "sam@example.com" {
		t.Errorf("step 2 = %+v", second)
	}
	if len(third.Images) != 1 || len(third.Blocks) != 1 || third.Blocks[0].Closed ||
		third.Blocks[0].Line != 14 {
		t.Errorf("step 3 = %+v", third)
	}
	if got := PlainMarkdown(first.Text); got !=
		"Install it on your phone: Shop beta (https://testflight.apple.com/join/abc)" {
		t.Errorf("PlainMarkdown = %q", got)
	}
}
