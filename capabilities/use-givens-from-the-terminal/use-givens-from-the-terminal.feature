Feature: Use Givens from the terminal
  As a developer in a terminal, or an agent working for one
  I want to list, run and check scenarios with plain commands
  So that agents and CI (automated checks on each push) can use the same files

  `list`, `run` and `check` print plain text, for agents and CI, and never open the TUI (a
  text UI: full screen, driven by the keyboard). A person at a terminal has the TUI and their
  own editor too; see work-in-the-terminal.

  Scenario: List scenarios
    Given a developer's terminal is in the sample shop's folder
    When they run "givens list"
    Then each scenario is printed as its path, like "checkout/card-declined", with its kind and status
    And "givens list --json" prints the same as JSON

  Scenario: Run a setup by name
    Given a developer's terminal is anywhere inside the sample shop's folder
    When they run "givens run checkout/card-declined"
    Then the setup runs, with its "your turn" lines

  Scenario: Check passes
    Given every file in the sample shop is valid
    When a developer runs "givens check"
    Then it prints "9 scenarios, 0 problems"
    And it exits with 0

  @error
  Scenario: Check finds a problem
    Given checkout.feature has the line "Check the order is pending" among the steps of "Card declined"
    When a developer runs "givens check"
    Then it prints "checkout/checkout.feature line 17: expected a step, a tag, a comment or a new scenario"
    And it exits with 1

  @error
  Scenario: Run a scenario that doesn't exist
    Given a developer's terminal is in the sample shop's folder
    When they run "givens run checkout/nope"
    Then it prints "There's no scenario checkout/nope"
    And it exits with 1

  Scenario: List by tag
    Given a developer's terminal is in the sample shop's folder
    When they run "givens list --tags @error"
    Then it prints the same scenarios the Error states view lists

  Scenario: Steps done by hand in a terminal
    Given "Pay in app" has only scripts/pay-in-app.md
    When a developer runs "givens run checkout/pay-in-app"
    Then it prints each step as plain text
    And it waits for Enter after each one, or f if it failed

  Scenario: Code blocks in a terminal
    Given "Pay in app" has only scripts/pay-in-app.md
    And scripts/pay-in-app.md has a fenced code block indented under step 2
    When a developer runs "givens run checkout/pay-in-app"
    Then each code block is printed on its own lines, with no fences and no indent
    And it can be selected and pasted as it is

  # `givens skill` writes the agent skill Givens ships to .claude/skills/givens/SKILL.md, where
  # Claude Code finds a project's skills. The skill tells an agent to read capabilities/GIVENS.md
  # first, to use `givens list`, `run` and `check` rather than the TUI, and to flag work with
  # `givens todo` and `givens done`. It also adds a short "## Givens" paragraph pointing to the
  # skill to the project's CLAUDE.md, which Claude Code reads at the start of every session: a
  # project with no CLAUDE.md gets one, and a Givens paragraph already there is replaced, with
  # the rest of the file left as it was. Running it again leaves the same files.
  Scenario: Apply Givens agent skill to current project
    Given a developer's terminal is in the sample shop's folder
    And the sample shop has no .claude/skills/givens/SKILL.md
    And the sample shop's CLAUDE.md has a paragraph of its own
    When they run "givens skill"
    Then .claude/skills/givens/SKILL.md is written, a skill named "givens"
    And CLAUDE.md has a "## Givens" paragraph after its own
    And it prints "Wrote .claude/skills/givens/SKILL.md and CLAUDE.md"
    And running "givens skill" again changes nothing
