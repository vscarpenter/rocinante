// Package github will feed the Comms panel by shelling out to the gh CLI on an
// interval, reading open pull requests and CI status.
//
// Deferred to a later session. Shelling out to gh reuses the existing token
// rather than managing raw API auth. See build spec section 6.
package github
