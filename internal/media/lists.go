package media

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// TryPage reads a page of a long list with read, and while it fails,
// tries again after each of PageWaits, as a page of tracks is, unless ctx
// ends. It returns the error of the last try.
func TryPage(ctx context.Context, read func(ctx context.Context) error) error {
	for try := 0; ; try++ {
		err := read(ctx)
		if err == nil || try == len(PageWaits) || ctx.Err() != nil {
			return err
		}
		select {
		case <-time.After(PageWaits[try]):
		case <-ctx.Done():
			return err
		}
	}
}

// ShortList is the error of a list of the user's library on a service, as
// their playlists or the artists they follow, that came in part: Got of
// its Total items, as the service counted them when the reading began, 0
// when it didn't say. The items that came go with it, for the app to show
// with a way to read the list again. Err tells why the reading stopped;
// Cut is set when it stopped at the most of a list the service gives,
// which reading again doesn't pass.
type ShortList struct {
	Got, Total int
	Cut        bool
	Err        error
}

func (e *ShortList) Error() string {
	what := "only " + strconv.Itoa(e.Got)
	if e.Total > e.Got {
		what += " of " + strconv.Itoa(e.Total)
	}
	return what + " came: " + e.Err.Error()
}

func (e *ShortList) Unwrap() error { return e.Err }

// Short returns the ShortList that err is, or wraps, if it is one.
func Short(err error) (*ShortList, bool) {
	var short *ShortList
	ok := errors.As(err, &short)
	return short, ok
}
