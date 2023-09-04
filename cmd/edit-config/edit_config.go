package edit_config

import (
	"context"
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/networkguild/netconf"
	"github.com/spf13/cobra"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/parallel"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/utils"
)

var (
	defaltOp string
	testOp   string
	file     string
	copy     bool
	files    [][]byte
)

func NewEditConfigCommand() *cobra.Command {
	editConfigCmd := &cobra.Command{
		Use:   "edit-config",
		Short: "Execute edit-config rpc",
		Long: `Execute edit-config rpc for editing device configuration.

# edit-config with test-option
netconf edit-config --host 192.168.1.1 --test-option test-then-set --default-operation none --file edit-config.xml

# edit-config without optional options
netconf edit-config --host 192.168.1.1 --file rpc <- directory used here (probably files should be prefixed with number)`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.ParseConfig(context.Background())
			if err != nil {
				log.Fatalf("Failed to init config, error: %v", err)
			}

			f, err := utils.ReadFilesFromUser(file)
			if err != nil {
				log.Fatalf("Failed to read rpc's, error: %v", err)
			}
			files = f

			if err := parallel.RunParallel(cfg.Devices, runEditConfig); err != nil {
				log.Fatalf("Failed to run netconf, error: %v", err)
			}
		},
	}
	flags := editConfigCmd.Flags()
	flags.StringVarP(&file, "file", "f", "", "stdin, file or directory containing xml files")
	flags.StringVarP(&defaltOp, "default-operation", "d", "merge", "default-operation, none|merge|remove")
	flags.StringVarP(&testOp, "test-option", "t", "", "test-option, test-then-set|set|test-only")
	flags.BoolVarP(&copy, "copy", "c", false, "run copy-config after rpc's")

	return editConfigCmd
}

func runEditConfig(device *config.Device, session *netconf.Session) error {
	ctx, cancel := context.WithTimeout(device.Ctx, 5*time.Minute)
	defer cancel()

	var (
		capabilities = session.ServerCapabilities()
		errorOpt     = netconf.WithErrorStrategy(netconf.StopOnError)
		candidate    = slices.Contains(capabilities, netconf.CandidateCapability)
		validate     = slices.Contains(capabilities, netconf.ValidateCapability)
		startup      = slices.Contains(capabilities, netconf.StartupCapability)
	)
	if slices.Contains(capabilities, netconf.RollbackOnErrorCapability) {
		errorOpt = netconf.WithErrorStrategy(netconf.RollbackOnError)
	}
	datastore := netconf.Running
	if candidate {
		datastore = netconf.Candidate
	}
	start := time.Now()
	for _, data := range files {
		if reply, err := session.Lock(ctx, datastore); err != nil {
			return err
		} else {
			device.Log.Debugf("Lock reply:\n%s", reply.Raw())
		}

		if reply, err := session.EditConfig(ctx,
			datastore,
			data,
			errorOpt,
			netconf.WithDefaultMergeStrategy(netconf.MergeStrategy(defaltOp)),
			netconf.WithTestStrategy(netconf.TestStrategy(testOp)),
		); err != nil {
			device.Log.Errorf("Failed to edit candidate config: %v", err)
			return err
		} else {
			device.Log.Debugf("Edit-config reply:\n%s", reply.Raw())
		}

		if validate {
			reply, err := session.Validate(ctx, datastore)
			if err != nil {
				return err
			}
			device.Log.Debugf("Validate reply:\n%s", reply.Raw())
		}

		if reply, err := session.Commit(ctx); err != nil {
			return err
		} else {
			device.Log.Debugf("Commit reply:\n%s", reply.Raw())
		}

		if reply, err := session.Unlock(ctx, datastore); err != nil {
			return err
		} else {
			device.Log.Debugf("Unlock reply:\n%s", reply.Raw())
		}
	}
	device.Log.Infof("Executed %d edit-config requests, took %.3f seconds", len(files), time.Since(start).Seconds())

	start = time.Now()
	if copy && startup && netconf.TestStrategy(testOp) != netconf.TestOnly {
		if _, err := session.CopyConfig(ctx, netconf.Running, netconf.Startup); err != nil {
			return err
		}
		device.Log.Infof("Executed copy-config request, took %.3f seconds", time.Since(start).Seconds())
	}
	return nil
}
