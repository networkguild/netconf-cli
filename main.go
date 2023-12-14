package main

import (
	"log"

	"github.com/networkguild/netconf-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
