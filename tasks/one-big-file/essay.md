# One Big File: A Scenario-Driven Workflow

Oct 24, 2026 · @Aaron

## The idea

Nearly all the code lives in one big file, and the feature files are its table of contents. Stories and scenarios sit on top, and each distinct When points to a named entry point in the big file.

A thin framework edge handles requests and non-functional concerns, then hands straight to that entry point. A few deliberate exceptions live in adjacent files. An agent does the navigating and debugging set-up, so the developer stays in flow.

The scenarios below describe the workflow itself, in any language, using one running example: discount codes at checkout.

## Glossary

| Term | Meaning |
| --- | --- |
| Story | The user need a feature file opens with: as a…, I want…, so that… |
| Scenario | One concrete example of a story's behaviour, written as Given / When / Then |
| Entry point | A named function in the big file for one distinct When; scenarios point to it by name, for example with a tag like `@entry:apply-discount-code` |
| Big file | The single file holding nearly all behaviour: entry points first, helpers below |
| Helper | A named function extracted from an entry point, kept below all entry points |
| Adjacent file | One of the few deliberate exceptions: data models, framework glue, third-party integrations |
| Edge | The thin framework layer (routes, handlers, non-functional wiring) that delegates straight to entry points |
| Sync check | A plain script confirming every scenario maps to an entry point and a test |

## Scenarios are the map

Every scenario points at where its implementation begins, and many scenarios can share one entry point.

```gherkin
Feature: Scenarios are the map
  As a developer
  I want every scenario to point at its implementation
  So that I go from intent to code in one hop

  Scenario: Each distinct When has a named entry point
    Given the scenario "Apply a valid code"
    And its When is "the customer applies a code"
    When I look for its implementation
    Then I find the entry point "apply discount code"
    And it lives in the big file

  Scenario: Several scenarios share one entry point
    Given the scenario "Apply a valid code"
    And the scenario "Reject an expired code"
    And both have the When "the customer applies a code"
    Then both point to "apply discount code"
    And no implementation is duplicated between them

  Scenario: Givens are fixtures and Thens are assertions
    Given a scenario with Givens, a When and Thens
    Then its Givens become test fixtures
    And its When becomes a call to the entry point
    And its Thens become assertions on the result

  Scenario: The big file reads in spec order
    Given the feature files list scenarios in a set order
    Then entry points appear in the big file in that order
    And reading the big file top to bottom follows the spec

  Scenario: Feature files work without step definitions
    Given no BDD runner executes the feature files
    Then they still act as bookmarks into the big file
    And there is no step-definition glue to maintain
```

## One big file

New behaviour goes into the big file by default, and code is written to be easy to delete.

```
Feature: One big file holds the implementation
  As a developer
  I want nearly all behaviour in one file
  So that I never hunt through a folder tree

  Scenario: New behaviour lands in the big file
    Given a new scenario with a new When
    When I implement it
    Then its entry point is added to the big file
    And no new file is created for it

  Scenario: Gnarly logic becomes a named helper
    Given an entry point has grown hard to read
    When I extract part of it
    Then the extracted part becomes a named helper
    And helpers live below all entry points

  Scenario: Helpers outgrowing entry points is a tripwire
    Given the helper section outweighs the entry points
    When I notice it
    Then I move a cohesive group to an adjacent file
    And I record why it moved

  Scenario: Retiring a story removes its code
    Given a story is retired
    When I delete its scenarios
    Then entry points no other scenario uses are deleted
    And helpers nothing calls any more are deleted
```

## A thin framework edge

The framework handles transport and non-functional concerns, stated as Gherkin Rules that point to NFR comments, and never owns behaviour.

```gherkin
Feature: A thin framework edge
  As a developer
  I want the framework kept at the edge
  So that it never owns behaviour

  Scenario: A route delegates straight to an entry point
    Given an HTTP route for applying a discount code
    When a request arrives
    Then the route parses the input
    And calls "apply discount code"
    And formats the result as a response
    And contains no business logic

  Scenario: Non-functional requirements are Rules, not tags
    Given a story needs customers to be signed in
    Then its feature file states that as a Rule
    And the Rule groups the scenarios it governs
    And no tag wires up framework behaviour

  Scenario: A Rule points to an NFR comment in the code
    Given the Rule "Only signed-in customers can apply codes"
    When I search the code for that Rule's name
    Then I find an NFR comment naming it
    And the comment sits on the code that enforces it
    And the entry point knows nothing about sign-in

  Scenario: The big file never depends on the framework
    Given the edge depends on the framework
    Then the big file has no framework dependency
    And entry points run without a server
```

## Deliberate exceptions

Only three kinds of code may leave the big file without a recorded reason, and dependencies are passed in as plainly as possible.

```gherkin
Feature: Deliberate exceptions
  As a developer
  I want a short, explicit list of what lives elsewhere
  So that splitting is a decision, not drift

  Scenario Outline: Allowed adjacent files
    Given code that is <kind>
    Then it may live in an adjacent file
    And the big file's header lists that file and why

    Examples:
      | kind                       |
      | a data model definition    |
      | framework glue at the edge |
      | a third-party integration  |

  Scenario: Any other split needs a reason
    Given code that fits none of the allowed kinds
    When I want to move it out of the big file
    Then I record the reason in the big file's header
    But without a reason it stays in the big file

  Scenario: Third-party dependencies are injected simply
    Given "apply discount code" needs a pricing service
    When it is called
    Then the service is passed in as an argument
    And no dependency-injection framework is involved
    And a test can pass a fake in its place
```

## Testing through entry points

Tests call the same entry points the scenarios name, so the test suite mirrors the spec one-to-one.

```gherkin
Feature: Testing through entry points
  As a developer
  I want tests to call the entry points scenarios name
  So that the test suite mirrors the spec

  Scenario: One export exposes every entry point
    Given the big file
    Then it exposes all entry points through a single export
    And tests reach the big file only through that export

  Scenario: Every scenario has a matching test
    Given the scenario "Reject an expired code"
    Then a test with the same name exists
    And it sets up the Givens
    And calls the entry point
    And checks the Thens

  Scenario: Tests need no running server
    Given dependencies are passed in
    When a test calls an entry point with fakes
    Then it runs without the framework edge

  Scenario: Entry points share no hidden state
    Given two tests call different entry points
    When they run in any order
    Then neither depends on state left by the other
```

## Agent-guided navigation

The agent moves the developer around the big file by name and sets up debugging, while a human can still find the way alone.

```gherkin
Feature: Agent-guided navigation
  As a developer
  I want an agent to move me around the big file
  So that I stay in the zone

  Scenario: Jump to a scenario's implementation
    Given I name the scenario "Reject an expired code"
    When I ask the agent to take me there
    Then it finds "apply discount code" by name
    And opens the big file at that entry point

  Scenario: The agent reads only the slice it needs
    Given the big file is large
    When the agent works on a scenario
    Then it searches for that entry point's bounds
    And reads that slice plus the helpers it calls
    And loads the whole file only when asked

  Scenario: Set up a debugging session for a scenario
    Given I want to debug "Reject an expired code"
    When I ask the agent to set it up
    Then breakpoints are set on the entry point and its helpers
    And the scenario's test runs under the debugger

  Scenario: File and line pointers are generated, never stored
    Given feature files refer to entry points by name
    When I want to jump to the code myself
    Then a file and line pointer is resolved from the name
    And no line numbers are written into feature files

  Scenario: A human can still find the way alone
    Given no agent is available
    When I search the big file for an entry point's name
    Then I land on its implementation
```

## Spec and code stay in sync

With no BDD runner, a plain script keeps the feature files honest by checking them against the big file and the tests.

```gherkin
Feature: Spec and code stay in sync
  As a developer
  I want a cheap, deterministic check that spec and code agree
  So that the feature files never quietly lie

  Scenario: Every scenario resolves
    Given the feature files, the big file and the tests
    When the sync check runs before commit or in CI
    Then it passes only if every scenario has an entry point
    And every scenario has a matching test

  Scenario: A missing entry point fails the check
    Given a scenario whose entry point does not exist
    When the sync check runs
    Then it fails and names the scenario

  Scenario: Orphaned entry points are flagged
    Given an entry point no scenario refers to
    When the sync check runs
    Then it reports the entry point as a deletion candidate

  Scenario: The check runs without the agent
    Given the sync check is a plain script
    Then it gives the same answer every time
    And costs no agent time or tokens
```

## What a language must support

Any language that meets these four properties will carry the workflow; the choice beyond that is taste.

```gherkin
Feature: What a language must support
  As a developer choosing a language
  I want the few properties this workflow depends on
  So that any language meeting them will do

  Scenario: Names resolve at call time
    Given an entry point calls a helper defined further down
    When the program runs
    Then the call works without forward declarations

  Scenario: Feedback without leaving the file
    Given I change one entry point
    When I want to see the effect
    Then I can reload or re-evaluate it in seconds
    And I do not restart the whole system

  Scenario: The agent can drive the debugger
    Given the agent knows an entry point's name
    When it sets up a debugging session
    Then it can set breakpoints by function name
    And it can do so from a command line

  Scenario: Large files stay responsive
    Given the big file has grown to thousands of lines
    Then the editor, type checker and language server stay usable
```
