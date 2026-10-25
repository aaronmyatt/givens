source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'open a terminal in your project'"'"'s folder'
when_you 'run givens new story <name> and fill in the As a, I want and So that lines in your editor'
then_check 'a folder for the new capability holds its .feature file'
then_check 'the story is listed when you run givens'
