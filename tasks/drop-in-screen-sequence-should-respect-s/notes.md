A When/Then following a previous suite of GivenWhenThen is awkwardly combined in the Drop In screen pulling them out of sequence. The Drop In flow should respect the scenario sequence.
```
@inferred
Scenario: She reads one post in full
  Given a visitor is looking at the feed
  When she chooses a post card
  Then a modal opens with the whole text, untruncated
  And it shows the builder's name, handle and project
  And it shows how long ago the post went up and how many cheers it has
  When she closes it
  Then the feed is as she left it
```

## Goal
The Drop in page keeps a scenario's steps in the order they're written: When, Then, When,
Then shows as four parts, not one When part and one Then part.

## Design
- The core's checklist is already in file order (manual Givens, then each step). The bug is
  the Mac page: it filters all non-Then items into one "When" part and all Thens into one
  "Then" part.
- Fix in the core, so Go and Swift steps check the same thing: the Drop in snapshot gets
  `parts`, each a kind ("given", "when" or "then") and the run of checklist items of that
  kind, in order. The page draws one numbered part per entry, after the setup's part 1.
- Note, attachments and "Record run" move to the foot, below the last part, since there can
  be more than one Then part now.
- Manual Givens ("First, …") stay one part at the top, as they come before any step.

## As built (2026-10-12)
- Spec: "Steps stay in the order they're written". The Given adds a second When and Then to
  Card declined in the sample shop's copy.
- Core: `Session.Parts()` and the Drop in snapshot's `parts`, in dropin_page.go. The
  checklist itself was already in file order; only its doc comments said otherwise.
- Mac: DropInPage draws part 1 (setup), then one part per entry in `parts`, numbered on.
  A part's number turns into a tick once all its items are marked. The note, the attached
  files and "Record run" sit in a foot below the last part.
- Not eyeballed. To check: add `When they choose "Try another card"` and `Then the card form
  opens again` to a scenario, drop in, and see four numbered parts in that order.
