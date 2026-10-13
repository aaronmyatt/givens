Feature: Read a capability
  As a developer getting to know part of an app
  I want to see a capability's story, images and scenarios together
  So that I understand what it's for and how it behaves before I change it

  Capabilities sit in the sidebar with their scenario counts. The middle column holds the story
  and its scenarios; the right-hand side shows the selected scenario's steps next to its setup
  script. Views in the sidebar, such as "Error states", list scenarios from every capability at
  once.

  Scenario: Read a story
    Given a developer has the sample shop project open in Givens
    When they select "Checkout"
    Then they see "As a returning shopper", "I want to pay with my saved card" and "So that checkout takes seconds"
    And its 5 scenarios are listed, with happy paths and error states marked differently
    And below the scenarios, about.md is shown with the checkout flow diagram

  Scenario: Read a scenario
    Given a developer has the sample shop project open in Givens
    When they select the "Card declined" scenario
    Then its steps are shown with their keywords lined up
    And its tags, @error and @timeout:5m, are shown above its steps
    And its setup script is shown next to the steps
    And they can choose "Copy" or "Drop in"

  @error
  Scenario: A scenario with no setup
    Given a developer has the sample shop project open in Givens
    And the "Empty cart" scenario has no setup yet
    When they select "Empty cart"
    Then its steps are shown as normal
    And in place of a setup they see "No setup yet"
    And they're offered "Create script" and "Write setup steps"

  @error
  Scenario: A story image that isn't on disk
    Given a developer has the sample shop project open in Givens
    And Checkout's about.md shows an image that isn't on disk
    When they select "Checkout"
    Then the rest of the story is shown as normal
    And where the image would be there's a placeholder naming "images/flow.svg"
