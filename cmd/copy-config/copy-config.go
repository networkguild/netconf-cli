package copy_config

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/networkguild/netconf"
	"github.com/spf13/cobra"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/parallel"
)

var (
	sourceFlag string
	targetFlag string
	sourceUrl  string
	targetUrl  string
)

func NewCopyConfigCommand() *cobra.Command {
	copyConfigCmd := &cobra.Command{
		Use:   "copy-config",
		Short: "Execute copy-config rpc",
		Long: `Execute copy-config rpc for copying configuration from one datastore to another.

# copy-config with test-option
netconf copy-config --host 192.168.1.1 --test-option test-then-set --default-operation none --file edit-config.xml

# copy-config without optional options
netconf copy-config --host 192.168.1.1 --file rpc <- directory used here (probably files should be prefixed with number)`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.ParseConfig(context.Background())
			if err != nil {
				log.Fatalf("Failed to init config, error: %v", err)
			}

			if err := parallel.RunParallel(cfg.Devices, runCopyConfig); err != nil {
				log.Fatalf("Failed to run netconf, error: %v", err)
			}
		},
	}
	flags := copyConfigCmd.Flags()
	flags.StringVarP(&sourceFlag, "source", "s", "", "source configuration datastore")
	flags.StringVarP(&targetFlag, "target", "t", "", "target configuration datastore to save config")
	flags.StringVarP(&sourceUrl, "source-url", "S", "", "source configuration url")
	flags.StringVarP(&targetUrl, "target-url", "T", "", "target configuration url to save config")
	copyConfigCmd.MarkFlagsMutuallyExclusive("source", "source-url")
	copyConfigCmd.MarkFlagsMutuallyExclusive("target", "target-url")

	return copyConfigCmd
}

func runCopyConfig(device *config.Device, session *netconf.Session) error {
	ctx, cancel := context.WithTimeout(device.Ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	var source any
	switch {
	case sourceFlag != "":
		source = netconf.Datastore(sourceFlag)
	case sourceUrl != "":
		source = netconf.URL(sourceUrl)
	default:
		return fmt.Errorf("no source specified")
	}

	var target any
	switch {
	case targetFlag != "":
		target = netconf.Datastore(targetFlag)
	case targetUrl != "":
		target = netconf.URL(targetUrl)
	default:
		return fmt.Errorf("no target specified")
	}

	if reply, err := session.CopyConfig(ctx, source, target); err != nil {
		return err
	} else {
		device.Log.Debugf("Copy-config reply:\n%s", reply.Raw())
	}

	device.Log.Infof("Executed copy-config request, took %.3f seconds", time.Since(start).Seconds())
	return nil
}
