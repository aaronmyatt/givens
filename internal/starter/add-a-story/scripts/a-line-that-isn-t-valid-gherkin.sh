source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'open a scenario in the editor'
when_you 'write a line among its steps that doesn'"'"'t start with Given, When, Then, And, But or *'
then_check 'the line is marked with the parser'"'"'s message'
then_check 'the file is saved anyway'
