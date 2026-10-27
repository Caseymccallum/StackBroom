// Package version carries the build identity of StackBroom.
package version

// Version is the application version. It is replaced at build time with:
//
//	go build -ldflags "-X github.com/Caseymccallum/StackBroom/internal/version.Version=0.2.0"
var Version = "0.2.0-dev"

// Name is the application name used in JSON output and the CLI banner.
const Name = "stackbroom"
