source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'move to a story with no scenarios yet in givens'
when_you 'press n and write its Given, When and Then steps in your editor'
then_check 'the scenario is in the capability'"'"'s .feature file'
then_check 'pressing s on it starts scripts/<slug>.sh'
