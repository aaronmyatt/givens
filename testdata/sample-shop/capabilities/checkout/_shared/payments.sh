# shellcheck shell=bash
# A fake payment provider. Only Checkout's setup scripts can call these helpers.

# @helper mock_payments [--decline | --challenge 3ds]
# Points the shop at a fake payment provider that approves, declines or challenges every card.
mock_payments() {
  case "${1:-}" in
    "") ok "mocked payments: every card is approved" ;;
    --decline) ok "mocked payments: every card is declined" ;;
    --challenge) ok "mocked payments: every card asks for a ${2:-3ds} check" ;;
    *) fail "mock_payments: unknown option $1" ;;
  esac
}
