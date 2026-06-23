// Package roci will pull a same-schema status file from Roci on EC2 across the
// tailnet on an interval, caching the last good read so a brief blip shows
// stale rather than vanishing.
//
// Deferred to the tailnet phase. The Tailscale transport must be confirmed
// before this is built; the recommendation is Tailscale SSH reading the file.
// See build spec sections 5 and 15.
package roci
