package version

import "runtime"

var (
	// Name is the name of the application
	Name = "heimdall"

	// Version defines the application version
	Version = "0.2.0"

	// Description is the application description
	Description = "darknet router"

	// Build will be overwritten automatically by the build system
	Build = "-dev"

	// GitCommit will be overwritten automatically by the build system
	GitCommit = "HEAD"
)

// BuildVersion returns the build version information including version, build and git commit
func BuildVersion() string {
	return Version + Build + " (" + GitCommit + ") " + runtime.GOOS + "/" + runtime.GOARCH
}

// FullVersion returns the build version information including version, build and git commit
func FullVersion() string {
	return Name + "/" + BuildVersion()
}
