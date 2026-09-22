package version

// Version is the application semver without a leading "v".
// Override at link time:
//
//	-ldflags "-X github.com/erikharutyunyan/double-pane/internal/version.Version=1.2.3"
var Version = "0.0.0-dev"

// GitCommit is optional short commit SHA for debugging (empty if not set).
var GitCommit = ""
