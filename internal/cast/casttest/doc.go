// Package casttest has a fake cast device for tests: a TLS server on the
// loopback address that speaks the Cast v2 protocol as a device does, with
// a default media receiver, and keeps the messages it gets; and an mDNS
// responder that answers for it. It frames and encodes its messages by
// itself, apart from package cast, so that a test checks one against the
// other.
package casttest
