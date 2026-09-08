package contexting

import (
	"runtime/debug"
	"strings"
)

// Version is set via ldflags for release archives. Tagged go installs use the
// module version embedded by the Go toolchain.
var Version = "dev"

func currentVersion() string {
	if Version != "" && Version != "dev" {
		return strings.TrimPrefix(Version, "v")
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}
