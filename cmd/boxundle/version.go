package main

import (
	"fmt"
	"runtime/debug"
)

// version は、goreleaser が -X main.version=<タグ> で埋める。go install で入れたものでは空。
var version string

// runVersion は boxundle version。
func runVersion(c *call) int {
	info, _ := debug.ReadBuildInfo()
	_, _ = fmt.Fprintf(c.stdout, "boxundle %s\n", versionString(version, info))
	return exitOK
}

// versionString は、埋めた版、無ければモジュールの版、それも無ければ (devel) を返す。
func versionString(embedded string, info *debug.BuildInfo) string {
	if embedded != "" {
		return embedded
	}
	if info != nil && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}
