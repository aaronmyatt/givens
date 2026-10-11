source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_shopper

when_you 'ask for a sign-in link and open it'
then_check "you're signed in"
