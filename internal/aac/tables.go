package aac

import "sync"

// initTables builds the tables a Decoder needs, once.
var initTables = sync.OnceFunc(func() {
	initBooks()
})
