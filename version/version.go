package version

import "fmt"

// Major and Minor are the stable components of the version. Update these when
// you make breaking or feature releases. The date-based patch (YYYYMMDD) is
// injected at build time into Full.
const (
	Major = 1
	Minor = 18
	// PatchDate uses YYYYMMDD format
	PatchDate = "20250911"
)

// Full is the complete version string. Keep this in sync with Major, Minor, and PatchDate.
var Full = fmt.Sprintf("%d.%d.%s", Major, Minor, PatchDate)
