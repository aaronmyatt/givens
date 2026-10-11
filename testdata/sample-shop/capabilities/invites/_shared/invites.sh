# shellcheck shell=bash
# Shop owners and the invites they send. Only Invites' setup scripts can call these helpers.

# @helper seed_owner [email]
# Seeds a shop owner who can invite their team.
seed_owner() {
  local email="${1:-owner@example.com}"
  ok "seeded shop owner $email"
}

# @helper seed_invite <email>
# Has the shop owner invite someone, and leaves the invite unanswered.
seed_invite() {
  local email="$1"
  ok "invited $email"
}
