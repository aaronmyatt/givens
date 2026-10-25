# Drop into a scenario

A Drop in starts from the TUI, with Enter on a scenario, or from `givens drop-in <key>`. Both
show the TUI's Drop in page: setup's output as it runs, then its cards, then the scenario's
Given steps over the checklist. The foot of the page lists the keys that work now.

## Offers and their keys

What Givens offers on the page, and the key for each. One key can mean two offers that are
never on show at once, like t for "Try again" and for "Trust".

| Offer | Key |
| --- | --- |
| Done, Continue | Enter |
| Failed | f |
| Open | o |
| Copy | c |
| Keep waiting, Stop | k, s |
| Record run | r |
| Reset state | R |
| Try again | t |
| Edit script, Edit setup | s |
| Edit teardown | E |
| Run setup anyway | u |
| Add teardown | A |
| Trust, Don't trust, Trust this project | t, d, T |
| Share runs | S |
| Leave, Cancel | y, n |

## The checklist

| Key | What it does |
| --- | --- |
| j k (or ↑ ↓), g G | move, wrapping at the ends; top, bottom |
| Enter | mark done (a Then step: passed) |
| p, f, x | pass, fail, clear |
| n | write the run's note in your editor |
| a | attach a file, by its path |

e edits the scenario at its line and s edits its setup, in `$VISUAL` or `$EDITOR`; while
setup runs long, s is Stop. c on a card copies its first code block, or else its first inline code. q or Esc
leaves, asking first when marks aren't recorded.
