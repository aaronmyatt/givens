source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'select a story with no scenarios yet'
when_you 'choose "New scenario" and write its Given, When and Then steps'
then_check 'the scenario is in the capability'"'"'s .feature file'
then_check 'scripts/<slug>.sh was created for it'
