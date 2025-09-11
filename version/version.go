package version

// Major and Minor are the stable components of the version. Update these when
// you make breaking or feature releases. The date-based patch (YYYYMMDD) is
// injected at build time into Full.
const (
	Major = 1
	Minor = 12
)

// Full should be set at build time using:
//
//	-ldflags "-X github.com/bevelwork/quick_ssm/version.Full=vMAJOR.MINOR.YYYYMMDD"
//
// During development builds this will be empty and callers can fall back as needed.
var Full = ""
