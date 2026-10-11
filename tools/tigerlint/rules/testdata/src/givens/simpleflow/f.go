package simpleflow

func init() {} // want "init is banned"

func flow() {
	defer func() {
		_ = recover() // want "recover is banned"
	}()
	goto end // want "goto is banned"
end:
	panic("no") // want "panic is banned"
}
