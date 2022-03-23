package main

import (
	"os"

	tmCli "github.com/arcology-network/3rd-party/tm/cli"
	"github.com/arcology-network/client-svc/server"
)

func main() {
	st := server.StartCmd

	cmd := tmCli.PrepareMainCmd(st, "BC", os.ExpandEnv("$HOME/monacos/client"))
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
