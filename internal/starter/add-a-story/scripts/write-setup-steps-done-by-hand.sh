source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'pick a scenario whose setup happens in a browser or an app'
when_you 'write the steps as a numbered list in scripts/<slug>.md'
then_check 'dropping in shows each step as a card'
