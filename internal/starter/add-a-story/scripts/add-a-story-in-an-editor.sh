source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
# Changes nothing: it only says what to do next.
first_you 'open your project'"'"'s capabilities folder in Givens'
when_you 'create capabilities/<name>/<name>.feature in your editor, starting with Feature: and a story'
then_check 'the story is listed in the sidebar within two seconds'
