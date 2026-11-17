package media

import "context"

// partialKey keys, in a context, the func that takes what a list of the
// user's library holds so far, of type T, a slice of its items.
type partialKey[T any] struct{}

// WithPartial returns a context that asks a list of the user's library
// being read, whose items are T, to hand over what it has read so far
// after each page, to take. It is called on the goroutine that reads the
// list, with a slice that is never written again.
func WithPartial[T any](ctx context.Context, take func(T)) context.Context {
	return context.WithValue(ctx, partialKey[T]{}, take)
}

// Partial returns the func that takes what a list of T has read so far,
// as WithPartial sets it, or nil when ctx has none.
func Partial[T any](ctx context.Context) func(T) {
	take, _ := ctx.Value(partialKey[T]{}).(func(T))
	return take
}
