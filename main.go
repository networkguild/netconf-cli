package main

import (
	"log"

	"github.devcloud.elisa.fi/netops/netconf-go/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
