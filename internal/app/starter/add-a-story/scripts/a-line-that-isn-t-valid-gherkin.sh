source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'open a scenario in your editor: press e on it in givens'
when_you 'write a line among its steps that doesn'"'"'t start with Given, When, Then, And, But or *'
then_check 'your editor opens again with the parser'"'"'s message at the top'
then_check 'the file isn'"'"'t saved until the line is fixed'
