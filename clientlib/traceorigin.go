package clientlib

import (
	"errors"

	"github.com/M-ERCURY/core/api/relayentry"
	"github.com/M-ERCURY/core/api/status"
	"github.com/M-ERCURY/poc/circuit"
)

func TraceOrigin(err error, circ circuit.T) (r *relayentry.T) {
	var e *status.T
	if errors.As(err, &e) {
		// this type of error can only be returned by relays
		// and will contain netstack origin info
		// so find responsible relay in circuit to provide
		// a more human-readable message (with url)
		if e.Origin != "" {
			for _, r0 := range circ {
				if e.Origin == r0.Pubkey.String() {
					r = r0
					break
				}
			}
		}
	}
	return
}
