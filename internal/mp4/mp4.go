// Package mp4 reads the audio of MP4 files, of the ISO base media file
// format (ISO/IEC 14496-12 and 14496-14), such as the AAC audio YouTube
// serves (itags 139 and 140): the packets of their first audio track, with
// their times, and seeking by their sample tables or segment indexes.
//
// It reads progressive files, whose moov describes every sample, whether
// it comes before the media data or after, and fragmented ones, whose moof
// boxes each describe the samples of the mdat after them, as YouTube's
// DASH files are, with a sidx to seek by.
//
// A Reader reads its io.ReadSeeker a little at a time, through a buffer,
// and seeks it only when it must: to go back, to skip far ahead, and to
// load a moov that comes after the media data. The sizes it reads are
// bounded, so that a corrupt one cannot allocate much memory, and
// malformed files make errors, never panics.
package mp4

import "errors"

// ErrFormat is wrapped by the errors of data that is not MP4, or is
// corrupt.
var ErrFormat = errors.New("mp4: invalid data")
