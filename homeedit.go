package main

// homeArrange is Home's own state: whether it shows as the user edits it,
// the shelf the keys moved last, which takes the focus at its new place,
// whether Reset asks, and the items of the shelves made of the library
// and the downloads (homeshelves.go).
type homeArrange struct {
	editing, resetting bool
	focus              string
	memos              map[string]*homeMemo
}
