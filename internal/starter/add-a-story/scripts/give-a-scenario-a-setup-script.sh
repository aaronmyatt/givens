source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'open a scenario whose setup script is empty'
when_you 'add the commands that set up its starting state, then when_you and then_check lines'
then_check '"Drop in" runs them and shows the steps as a checklist'
