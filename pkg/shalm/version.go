package shalm

import (
	"github.com/Masterminds/semver/v3"
)

var version = "latest"

var kubeVersion = "v1.17.0"

var kubeSemver, _ = semver.NewVersion(kubeVersion)

// Version -
func Version() string {
	return version
}

// DockerTag -
func DockerTag() string {
	v, err := semver.NewVersion(version)
	if err != nil || len(v.Prerelease()) > 0 {
		return "latest"
	}
	return version
}
