source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'open your project'"'"'s capabilities folder in Givens'
when_you 'choose "New story" and fill in the As a, I want and So that lines'
then_check 'a folder for the new capability holds its .feature file'
then_check 'the story is listed in the sidebar'
