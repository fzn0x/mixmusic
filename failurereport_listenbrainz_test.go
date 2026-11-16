package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// TestFailureReportLeavesOutListenBrainz leaves the ListenBrainz user name
// out of the details of a failure, as Last.fm's.
func TestFailureReportLeavesOutListenBrainz(t *testing.T) {
	pr := privacyOf(app.AccountsStatus{ListenBrainz: app.ListenBrainzAccount{Name: "robinbrainz"}}, "/home/robin")
	if got := pr.clean("submitting the listens of robinbrainz failed"); strings.Contains(got, "robinbrainz") {
		t.Errorf("the details hold the name: %q", got)
	}
}
