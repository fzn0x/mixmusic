package stream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
)

const (
	// rangeFirst is the size of the first block a Ranges asks for, and of a
	// block after a seek; rangeMost the size blocks read one after another
	// grow to, twice the one before each time.
	rangeFirst = 64 << 10
	rangeMost  = 1 << 20
	// rangesKept is how many of the last blocks a Ranges keeps, for a
	// reader that reads a little back.
	rangesKept = 2
)

// Ranges reads the audio of a video a block at a time, as it is read: a
// read beyond what came asks the proxy for the next block alone, with a
// range request, so that a reader that seeks reads only the stretches it
// reads, never the rest of the file, as a Source would in the background.
// It reads stretches of a song's audio to time its lyrics to its music
// video (#565), and of the video's. Its bytes come as the proxy serves
// them: from what is kept, the cache or YouTube; a block that is the whole
// file goes to the cache, as any whole download does. Its methods but
// Close are not safe for concurrent use.
type Ranges struct {
	p       *Proxy
	videoID string
	ctx     context.Context
	cancel  context.CancelFunc
	closed  atomic.Bool

	size, pos int64
	// blocks are the last blocks that came, the latest last, and next the
	// size of the block asked for after the latest.
	blocks []rangeBlock
	next   int64
}

// rangeBlock is bytes of the audio, from off.
type rangeBlock struct {
	off  int64
	data []byte
}

// OpenRanges opens the audio of a video, as Open does, to read it a block
// at a time: it returns once the proxy answered the first block, with why
// it couldn't, in plain words, as Failure tells. Close drops it.
func (p *Proxy) OpenRanges(ctx context.Context, videoID string) (*Ranges, error) {
	ctx, cancel := context.WithCancel(ctx)
	r := &Ranges{p: p, videoID: videoID, ctx: ctx, cancel: cancel, size: -1, next: rangeFirst}
	if _, err := r.block(0); err != nil {
		cancel()
		return nil, err
	}
	return r, nil
}

// Read reads the audio from where the reader is.
func (r *Ranges) Read(b []byte) (int, error) {
	if r.closed.Load() {
		return 0, errClosed
	}
	if r.pos >= r.size {
		return 0, io.EOF
	}
	k, err := r.block(r.pos)
	if err != nil {
		return 0, err
	}
	n := copy(b, k.data[r.pos-k.off:])
	r.pos += int64(n)
	return n, nil
}

// Seek moves where Read reads.
func (r *Ranges) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += r.pos
	case io.SeekEnd:
		offset += r.size
	default:
		return 0, errors.New("stream: invalid whence")
	}
	if offset < 0 {
		return 0, errors.New("stream: negative position")
	}
	r.pos = offset
	return offset, nil
}

// Close stops what the reader asks for, and drops what came.
func (r *Ranges) Close() error {
	r.closed.Store(true)
	r.cancel()
	return nil
}

// block returns the block that holds the byte at off, asking the proxy for
// it unless one that came does: the next block after the latest is twice
// as large, up to rangeMost, as a reader that reads on reads more.
func (r *Ranges) block(off int64) (rangeBlock, error) {
	for _, k := range r.blocks {
		if off >= k.off && off < k.off+int64(len(k.data)) {
			return k, nil
		}
	}
	n := int64(rangeFirst)
	if last := len(r.blocks) - 1; last >= 0 && r.blocks[last].off+int64(len(r.blocks[last].data)) == off {
		n = r.next
	}
	r.next = min(2*n, rangeMost)
	k, err := r.fetch(off, n)
	if err != nil {
		return rangeBlock{}, err
	}
	if len(r.blocks) == rangesKept {
		r.blocks = append(r.blocks[:0], r.blocks[1:]...)
	}
	r.blocks = append(r.blocks, k)
	return k, nil
}

// fetch asks the proxy for n bytes of the audio from off.
func (r *Ranges) fetch(off, n int64) (rangeBlock, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, Path(r.videoID), nil)
	if err != nil {
		return rangeBlock{}, err
	}
	req.Header.Set("Range", "bytes="+strconv.FormatInt(off, 10)+"-"+strconv.FormatInt(off+n-1, 10))
	w := &blockWriter{header: http.Header{}}
	r.p.ServeHTTP(w, req)
	if err := r.ctx.Err(); err != nil {
		return rangeBlock{}, err
	}
	switch w.status {
	case http.StatusOK:
		// The whole audio, as for a file smaller than the block.
		r.size = int64(w.body.Len())
		if off >= r.size {
			return rangeBlock{}, io.EOF
		}
		return rangeBlock{off: 0, data: w.body.Bytes()}, nil
	case http.StatusPartialContent:
		from, size, ok := contentRange(w.header.Get("Content-Range"))
		if !ok || from != off || w.body.Len() == 0 {
			return rangeBlock{}, errLoad
		}
		r.size = size
		return rangeBlock{off: off, data: w.body.Bytes()}, nil
	case http.StatusRequestedRangeNotSatisfiable:
		return rangeBlock{}, io.EOF
	}
	if why := r.p.Failure(r.videoID); why != "" {
		return rangeBlock{}, errors.New(why)
	}
	return rangeBlock{}, errLoad
}

// blockWriter is the response the proxy writes a block to.
type blockWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *blockWriter) Header() http.Header { return w.header }

func (w *blockWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *blockWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	if w.status != http.StatusOK && w.status != http.StatusPartialContent {
		return len(b), nil // the text of an error
	}
	return w.body.Write(b)
}
