Feature: Trust a project
  As a developer opening a project someone else wrote
  I want Givens to ask before it runs any of the project's scripts
  So that opening a cloned repo can't run code I haven't looked at

  Trust is tied to the project's folder path and stored in Givens's own app data on that Mac,
  next to the recent projects list. It's never saved in the project, so a cloned repo can't mark
  itself as trusted, and a re-clone or moved folder asks again. Reading, searching and copying
  work without trust. Only running scripts needs it.

  Scenario: Asked before the first run
    Given a developer has never run anything from the sample shop
    When they drop in to "Card declined"
    Then they see "Run scripts from this project?" with the project's path
    And they're offered "Trust" and "Don't trust"
    And nothing runs until they choose

  Scenario: Trust is remembered
    Given a developer chose "Trust" for the sample shop before
    When they drop in to "Card declined"
    Then setup starts without asking

  @error
  Scenario: Choose not to trust
    Given a developer chose "Don't trust" for the sample shop
    When they drop in to "Card declined"
    Then the Drop in screen opens with the scenario's steps and setup script, but nothing in the script runs
    And they see "Scripts are off for this project", and "Copy" still works
    And they're offered "Trust this project"

  @error
  Scenario: Moved or re-cloned project
    Given a developer trusted the sample shop at ~/code/shop
    When they open a fresh clone of it at ~/code/shop-2 and drop in
    Then they're asked "Run scripts from this project?" again
