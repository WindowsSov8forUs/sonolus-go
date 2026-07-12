package main

import (
	"fmt"
	"runtime/debug"
	"strings"
)

type buildMetadata struct {
	version string
	commit  string
	date    string
}

func currentBuildMetadata() buildMetadata {
	info, _ := debug.ReadBuildInfo()
	return resolveBuildMetadata(version, commit, date, info)
}

func resolveBuildMetadata(version, commit, date string, info *debug.BuildInfo) buildMetadata {
	metadata := buildMetadata{version: version, commit: commit, date: date}
	if info == nil {
		return metadata
	}

	if metadata.version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		metadata.version = strings.TrimPrefix(info.Main.Version, "v")
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if metadata.commit == "unknown" {
				metadata.commit = setting.Value
			}
		case "vcs.time":
			if metadata.date == "unknown" {
				metadata.date = setting.Value
			}
		}
	}
	return metadata
}

func (metadata buildMetadata) String() string {
	details := make([]string, 0, 2)
	if metadata.commit != "" && metadata.commit != "unknown" {
		details = append(details, "commit "+metadata.commit)
	}
	if metadata.date != "" && metadata.date != "unknown" {
		details = append(details, "built "+metadata.date)
	}
	if len(details) == 0 {
		return fmt.Sprintf("sonolus-go %s", metadata.version)
	}
	return fmt.Sprintf("sonolus-go %s (%s)", metadata.version, strings.Join(details, ", "))
}
