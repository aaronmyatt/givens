Feature: Copy a setup script
  As a developer in a terminal, or handing work to a colleague or an agent
  I want to copy a scenario's setup as something I can paste and run
  So that I can reach that state wherever I'm working

  "Copy" puts `bash capabilities/<capability>/scripts/<slug>.sh` on the clipboard, to run from
  the git repo's root folder. The kit moves into the repo root before running anything, so the
  command works from any folder inside it. The script runs as written, kit and helpers included,
  so the copy never drifts from the files.

  Copy is one of Givens's offers on the selected scenario. From a search, it copies the first
  scenario found, and leaves what's selected alone.

  Scenario: Copy
    Given a developer has the sample shop project open in Givens
    And "Card declined" is selected
    When they choose "Copy"
    Then the clipboard holds "bash capabilities/checkout/scripts/card-declined.sh"
    And running it from the project's root folder sets up the scenario

  Scenario: Copy from search
    Given a developer has the sample shop project open in Givens
    And "Sign in" is selected
    When they search for "card declined" and copy the first scenario found
    Then the clipboard holds "bash capabilities/checkout/scripts/card-declined.sh"
    And "Sign in" is still selected

  Scenario: Capabilities folder isn't at the root
    Given the sample shop's capabilities folder is at docs/capabilities in its git repo
    When a developer chooses "Copy" on "Card declined"
    Then the clipboard holds "bash docs/capabilities/checkout/scripts/card-declined.sh"
    And running it from the repo's root folder sets up the scenario

  Scenario: Run from a subfolder
    Given a developer's terminal is in the sample shop's src folder
    When they run "bash ../capabilities/checkout/scripts/card-declined.sh"
    Then setup runs the same as it would from the root folder

  Scenario: Copy steps done by hand
    Given "Pay in app" has only scripts/pay-in-app.md
    And "Pay in app" is selected
    When a developer chooses "Copy"
    Then the clipboard holds the markdown of pay-in-app.md
