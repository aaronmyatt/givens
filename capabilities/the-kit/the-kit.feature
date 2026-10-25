Feature: The kit
  As a developer writing or running setup scripts
  I want the kit to behave the same when Givens runs a script and when I run it myself
  So that a script I try by hand acts the same in a Drop in

  The kit is `capabilities/_shared/givens.sh`: one file, written for bash 3.2, copied unchanged
  into every project. Every setup script sources it on its first line. See about.md for its
  functions and variables. Each scenario here maps to one bats test (bats is a test runner for
  bash). None need Givens itself. `--when` doesn't run setup again: it assumes the developer is
  mid-flow, with setup already done.

  @error
  Scenario: Strict mode stops at the first failure
    Given a setup script's second command fails
    When a developer runs the script in a terminal
    Then setup stops at that command
    And the lines after it don't run
    And the script exits with a non-zero code

  Scenario: "Your turn" heading prints once
    Given a setup script calls first_you, then when_you twice, then then_check
    When a developer runs it in a terminal
    Then "Your turn" is printed once, just before the first_you line

  Scenario: Quiet mode
    Given the same script as kit-your-turn
    When it runs with GIVENS_QUIET=1
    Then no "Your turn" heading or first_you, when_you or then_check lines are printed
    And ok, warn and fail lines are still printed

  Scenario: No colour when not in a terminal
    Given a setup script calls ok "seeded shopper"
    When its output is sent to a file
    Then the file holds "✓ seeded shopper" with no colour codes

  @error
  Scenario: A missing command
    Given a setup script has need shopctl "brew install shop/tap/shopctl"
    And shopctl isn't installed
    When a developer runs the script in a terminal
    Then it prints "✗ needs shopctl: brew install shop/tap/shopctl"
    And nothing after the need line runs

  @error
  Scenario: fail stops setup
    Given a setup script calls fail "No database found"
    When it runs
    Then it prints "✗ No database found"
    And the script exits with a non-zero code

  Scenario: warn carries on
    Given a setup script calls warn "Mailpit isn't running", then ok "seeded shopper"
    When it runs
    Then both lines are printed
    And the script exits with 0

  Scenario: When mode
    Given a setup script ends with when_you "run this: $(when_command)"
    And it has if when_mode; then …; exit 0; fi near the top
    When a developer runs it, then runs the command it prints
    Then the first run prints "When you run this: bash capabilities/…/<slug>.sh --when"
    And the second run does only the When part, without rerunning setup, then exits

  Scenario: A step by hand in a terminal
    Given a setup script has a by_hand step for "DigitalOcean console" with a snippet
    When a developer runs it in a terminal
    Then it prints "By hand, in DigitalOcean console:" followed by the snippet
    And it waits with "Press Enter when done, or type f if it failed"
    And pressing Enter carries on with the rest of setup

  @error
  Scenario: A step by hand fails in a terminal
    Given a setup script is waiting at a by_hand step in a terminal
    When the developer types f and presses Enter
    Then it prints "✗ Stopped at a step done by hand"
    And the script exits with a non-zero code

  Scenario: Ask in a terminal
    Given a setup script has ask INVITE_CODE "Paste the invite code"
    When a developer runs it in a terminal and types "K7Q2"
    Then the rest of the script sees INVITE_CODE as "K7Q2"

  @error
  Scenario: Nobody at the keyboard
    Given a setup script has a by_hand step
    When it runs with no terminal and not from Givens, as in CI
    Then it prints "✗ by_hand needs someone at the keyboard"
    And the script exits with a non-zero code instead of waiting
