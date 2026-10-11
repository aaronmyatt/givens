![Checkout flow](images/flow.svg)

Payments go through the mock gateway in `_shared/payments.sh`. Card declines and 3-D Secure
challenges are switched on with `mock_payments --decline` and `mock_payments --challenge 3ds`.
