Feature: Add a story
  As a Givens user
  I want to add a story
  So that I can document how the project works for myself and others

  Givens writes this capability into every new capabilities folder. It documents adding stories
  and scenarios as a story and scenarios, so each project starts with a worked example of the
  format. Its setup scripts change nothing: they only print what to do next. Rewrite this
  capability or delete it once your own stories are in place.

  Scenario: Add a story in Givens
    Given a user has their project's capabilities folder open in Givens
    When they choose "New story"
    And they fill in the As a, I want and So that lines
    Then a folder for the new capability is created, holding its .feature file
    And the story is listed in the sidebar

  Scenario: Add a story in an editor
    Given a user has their project's capabilities folder open in Givens
    When they create a folder in capabilities with a .feature file of the same name inside it
    Then the story is listed in the sidebar within two seconds

  # An agent can add stories the same way; GIVENS.md tells it the format.

  Scenario: Add a scenario to a story
    Given a user has a story with no scenarios yet
    When they choose "New scenario" and write its Given, When and Then steps
    Then the scenario is added to the capability's .feature file
    And a setup script named after the scenario is created in its scripts folder

  Scenario: Give a scenario a setup script
    Given a user has a scenario whose setup script is empty
    When they add the commands that put the app into the scenario's starting state
    And they end the script with the steps to take next
    Then "Drop in" runs those commands and shows the scenario's steps as a checklist

  Scenario: Write setup steps done by hand
    Given a user has a scenario whose setup is done in a browser or an app
    When they write the steps as a numbered list in scripts/<slug>.md
    Then "Drop in" shows each step as a card

  @error
  Scenario: A line that isn't valid Gherkin
    Given a user is writing a scenario
    When they write a step that doesn't start with Given, When, Then, And, But or *
    Then the line is marked with the parser's message
    And the file is saved anyway
