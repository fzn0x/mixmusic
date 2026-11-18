package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/autoeq"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The correction of headphones (#616): AutoEq's profiles, fetched as the
// list opens and a profile is picked, and kept in the data folder's
// autoeq, and the ParametricEQ.txt files Import file… reads.

// newAutoEq returns the client of AutoEq's results, which keeps them in
// the data folder's autoeq.
func newAutoEq(opts Options, userAgent string) *autoeq.Client {
	c := autoeq.New(filepath.Join(opts.DataDir, "autoeq"), userAgent)
	if opts.AutoEqURL != "" {
		c.Base = opts.AutoEqURL
	}
	return c
}

// correctionOf is the engine's correction of the settings' h, which plays
// whether the equalizer and Sound effects are on or not: it corrects the
// headphones, as balance and mono serve how the user hears.
func correctionOf(h settings.Headphones) audio.Correction {
	c := audio.Correction{Filters: []audio.CorrectionFilter{}}
	if !h.On() {
		return c
	}
	c.Preamp = h.Preamp
	for _, f := range h.Filters {
		c.Filters = append(c.Filters, audio.CorrectionFilter{Type: f.Type, Hz: f.Fc, Gain: f.Gain, Q: f.Q})
	}
	return c
}

// HeadphonesIndex returns AutoEq's list of profiles: the one kept, checked
// again once a week, and, without the network, why it couldn't be, in
// the windows' words.
func (p *Preferences) HeadphonesIndex(ctx context.Context) (autoeq.List, error) {
	list, err := p.a.autoEq.Index(ctx)
	if err != nil {
		return list, headphonesWords("fetching AutoEq's index", err)
	}
	if list.Offline != nil {
		list.Offline = headphonesWords("checking AutoEq's index", list.Offline)
	}
	return list, nil
}

// HeadphonesProfile returns the correction of AutoEq's profile e: the one
// kept, or the one on GitHub, which is kept once it reads.
func (p *Preferences) HeadphonesProfile(ctx context.Context, e autoeq.Entry) (settings.Headphones, error) {
	h, err := p.a.autoEq.Profile(ctx, e)
	if err != nil {
		return h, headphonesWords("fetching AutoEq's profile of "+e.Path, err)
	}
	return h, nil
}

// HeadphonesKept reports whether AutoEq's profile e is kept, and plays
// without the network.
func (p *Preferences) HeadphonesKept(e autoeq.Entry) bool { return p.a.autoEq.Kept(e) }

// ImportHeadphones reads the ParametricEQ.txt at path, strictly, as a
// correction named after the file: AutoEq's "Model ParametricEQ.txt" is
// named Model.
func (p *Preferences) ImportHeadphones(path string) (settings.Headphones, error) {
	f, err := os.Open(path)
	if err != nil {
		return settings.Headphones{}, err
	}
	defer f.Close()
	// A file far bigger than a ParametricEQ.txt isn't read whole: what is
	// read of it is too big.
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return settings.Headphones{}, err
	}
	h, err := settings.ParseParametricEQ(data)
	if err != nil {
		return h, headphonesWords("reading "+filepath.Base(path), err)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if model := strings.TrimSuffix(name, " ParametricEQ"); model != "" {
		name = model
	}
	h.ID, h.Name = settings.HeadphonesFile, name
	return h, nil
}

// headphonesWords is err, of doing what, as the windows say it: why a
// ParametricEQ.txt doesn't read, line by line, or why AutoEq's server
// didn't give it. The logs keep the English.
func headphonesWords(doing string, err error) error {
	english := doing + ": " + err.Error()
	var line *settings.ParametricEQError
	var many *settings.TooManyFiltersError
	var status *autoeq.StatusError
	switch {
	case errors.As(err, &line):
		l := *line
		return wordedLater(english, func() string { return lineWords(l) }, err)
	case errors.As(err, &many):
		count := many.Count
		return wordedLater(english, func() string { return locales.EqualizerHeadphonesTooMany(count, settings.MaxHeadphoneFilters) }, err)
	case errors.Is(err, settings.ErrNoFilters):
		return wordedLater(english, locales.EqualizerHeadphonesNoFilters, err)
	case errors.Is(err, settings.ErrParametricEQSize), errors.Is(err, autoeq.ErrTooBig):
		return wordedLater(english, locales.EqualizerHeadphonesTooBig, err)
	case errors.Is(err, autoeq.ErrEmptyIndex):
		return wordedLater(english, locales.EqualizerHeadphonesEmptyList, err)
	case errors.As(err, &status) && status.Code == http.StatusNotFound:
		return wordedLater(english, locales.EqualizerHeadphonesGone, err)
	case errors.As(err, &status):
		code := fmt.Sprint(status.Code, " ", http.StatusText(status.Code))
		return wordedLater(english, func() string { return locales.EqualizerHeadphonesServer(strings.TrimSpace(code)) }, err)
	}
	// The network's and the system's errors, which ErrorText says.
	return wordedLater(english, func() string { return ErrorText(err) }, err)
}

// lineWords is why the line of e doesn't read, in the windows' words.
func lineWords(e settings.ParametricEQError) string {
	switch e.Why {
	case settings.UnknownFilter:
		return locales.EqualizerHeadphonesLineType(e.Line, e.Value)
	case settings.MissingParameter:
		return locales.EqualizerHeadphonesLineMissing(e.Line, e.Value)
	case settings.ParameterTwice:
		return locales.EqualizerHeadphonesLineTwice(e.Line, e.Value)
	case settings.OutOfRange:
		lo, hi := settings.ParameterRange(e.Value)
		return locales.EqualizerHeadphonesLineRange(e.Line, e.Value, lo, hi)
	case settings.Bandwidth:
		return locales.EqualizerHeadphonesLineBandwidth(e.Line)
	case settings.LeftOver:
		return locales.EqualizerHeadphonesLineLeft(e.Line, e.Value)
	}
	return locales.EqualizerHeadphonesLineNot(e.Line)
}
