package domain

import "time"

// EasternTZ is America/New_York, the canonical time zone for all date
// computations in tdx. UFL's TeamDynamix tenant bills on Eastern time, so
// "this week" and "today" must be computed there regardless of laptop clock.
//
// The embedded tzdata import in cmd/tdx/main.go guarantees this load succeeds
// even on minimal container images without system tzdata.
var EasternTZ *time.Location

func init() {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic("tdx: failed to load America/New_York timezone: " + err.Error())
	}
	EasternTZ = loc
}
