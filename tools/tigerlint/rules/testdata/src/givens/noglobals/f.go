package noglobals

import "errors"

var counter int // want "package-level variable counter"

var ErrMissing = errors.New("missing")

var _ = counter

// The table is read-only after start-up.
var table = map[string]int{"a": 1} //tiger:allow-global read-only lookup

const limit = 3
