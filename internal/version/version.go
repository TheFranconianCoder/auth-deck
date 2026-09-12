package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// buildVersion can be set at build time with:
//
//	go build -ldflags "-X github.com/TheFranconianCoder/auth-deck/internal/version.buildVersion=$(git describe --tags --always --dirty)"
//
// It is empty for builds installed via `go install ...@version` and mise, where
// the module version is embedded in the build info instead.
var buildVersion string

// String returns the most specific version available: an explicit ldflags
// override, the module version embedded by `go install`/mise, or the VCS
// revision for local builds.
func String() string {
	if buildVersion != "" {
		return buildVersion
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	revision, modified := vcsInfo(info)
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		if modified {
			revision += "-dirty"
		}
		return revision
	}

	return "dev"
}

// Full returns a multi-field description used by the -version flag.
func Full() string {
	info, ok := debug.ReadBuildInfo()
	goVersion := runtime.Version()
	if ok && info.GoVersion != "" {
		goVersion = info.GoVersion
	}

	var b strings.Builder
	fmt.Fprintf(&b, "auth-deck %s\n", String())
	fmt.Fprintf(&b, "go: %s\n", goVersion)
	fmt.Fprintf(&b, "platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	if revision, modified := vcsInfo(info); revision != "" && ok {
		fmt.Fprintf(&b, "\ncommit: %s", revision)
		if modified {
			b.WriteString(" (dirty)")
		}
	}
	return b.String()
}

func vcsInfo(info *debug.BuildInfo) (revision string, modified bool) {
	if info == nil {
		return "", false
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	return revision, modified
}
