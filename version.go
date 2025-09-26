package main

// Build-time variables (set via -ldflags)
var (
	Version = "dev"
)

// GetVersion returns the build-time version
func GetVersion() string {
	return Version
}
