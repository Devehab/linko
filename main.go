// Command linko turns a locally running service into a public HTTPS URL using
// Cloudflare Tunnel.
package main

import "github.com/ibtkrgo/linko/cmd"

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=$(git describe --tags)"
var version = "dev"

func main() {
	cmd.Execute(version)
}
