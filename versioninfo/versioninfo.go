package versioninfo

import (
	"fmt"
	"runtime"
)

var (
	// NAME .
	NAME = "Eru-agent"
	// VERSION .
	VERSION = "unknown"
	// REVISION .
	REVISION = "HEAD"
	// BUILTAT .
	BUILTAT = "now"
)

// VersionString .
func VersionString() string {
	version := ""
	version += fmt.Sprintf("Version:        %s\n", VERSION)
	version += fmt.Sprintf("Git hash:       %s\n", REVISION)
	version += fmt.Sprintf("Built:          %s\n", BUILTAT)
	version += fmt.Sprintf("Golang version: %s\n", runtime.Version())
	version += fmt.Sprintf("OS/Arch:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return version
}
