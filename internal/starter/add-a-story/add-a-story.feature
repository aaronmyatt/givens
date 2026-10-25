Feature: Add a story
  As a Givens user
  I want to add a story
  So that I can document how the project works for myself and others

  Givens writes this capability into every new capabilities folder. It documents adding stories
  and scenarios as a story and scenarios, so each project starts with a worked example of the
  format. Its setup scripts change nothing: they only print what to do next. Rewrite this
  capability or delete it once your own stories are in place.

  Scenario: Add a story with givens new story
    Given a user's terminal is in their project's folder
    When they run "givens new story <name>"
    And they fill in the As a, I want and So that lines in their editor
    Then a folder for the new capability is created, holding its .feature file
    And the story is listed when they run "givens"

  Scenario: Add a story in an editor
    Given a user has their project open in Givens, with "givens"
    When they create a folder in capabilities with a .feature file of the same name inside it
    Then the story is listed within two seconds

  # An agent can add stories the same way; GIVENS.md tells it the format.

  Scenario: Add a scenario to a story
    Given a user has a story with no scenarios yet
    When they press "n" on it in Givens and write its Given, When and Then steps
    Then the scenario is added to the capability's .feature file
    And pressing "s" on it starts a setup script named after it, in its scripts folder

  Scenario: Give a scenario a setup script
    Given a user has a scenario whose setup script is empty
    When they add the commands that put the app into the scenario's starting state
    And they end the script with the steps to take next
    Then dropping in runs those commands and shows the scenario's steps as a checklist

  Scenario: Write setup steps done by hand
    Given a user has a scenario whose setup is done in a browser or an app
    When they write the steps as a numbered list in scripts/<slug>.md
    Then dropping in shows each step as a card

  @error
  Scenario: A line that isn't valid Gherkin
    Given a user is writing a scenario
    When they write a step that doesn't start with Given, When, Then, And, But or *
    Then their editor opens again with the parser's message at the top
    And the file isn't saved until the line is fixed
