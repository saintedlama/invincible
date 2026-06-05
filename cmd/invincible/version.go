package main

import "runtime/debug"

var version = "dev"

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return version + " (" + s.Value + ")"
		}
	}
	return version
}
