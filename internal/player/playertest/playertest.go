// Package playertest holds doubles of the player's interfaces that do
// nothing, for tests, as net/http/httptest holds HTTP's: Services, Stream
// and Device. A test's fake embeds one and overrides only the methods its
// test needs, so that a method added to an interface changes the double
// here and only the fakes that care, not every fake.
//
// Each double has its interface's methods only: the optional interfaces
// the player looks for, as player.Network on its services, a fake adds
// itself, so that embedding a double never changes what a fake is.
package playertest
