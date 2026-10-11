package forbiddenimports

import (
	"reflect" // want `import "reflect" is banned`
	"strings"
	"unsafe" // want `import "unsafe" is banned`
)

var _ = reflect.TypeOf
var _ = strings.ToUpper
var _ = unsafe.Sizeof(0)
