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

var opts struct {
	defaltOp string
	testOp   string
	file     string
	copy     bool
}

var files [][]byte

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

			f, err := utils.ReadFilesFromUser(opts.file)
			if err != nil {
				log.Fatalf("Failed to read rpc's, error: %v", err)
			}
			files = f

			if err := parallel.RunParallel(cfg.Devices, runEditConfig); err != nil {
				log.Fatalf("Failed to execute edit-config")
			}
		},
	}
	flags := editConfigCmd.Flags()
	flags.StringVarP(&opts.file, "file", "f", "", "stdin, file or directory containing xml files")
	flags.StringVarP(&opts.defaltOp, "default-operation", "d", "merge", "default-operation, none|merge|remove")
	flags.StringVarP(&opts.testOp, "test-option", "t", "", "test-option, test-then-set|set|test-only")
	flags.BoolVarP(&opts.copy, "copy", "c", false, "run copy-config after rpc's")

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
		device.Log.Debugf("Locking %s datastore", datastore)
		if err := session.Lock(ctx, datastore); err != nil {
			return err
		}

		if err := session.EditConfig(ctx,
			datastore,
			data,
			errorOpt,
			netconf.WithDefaultMergeStrategy(netconf.MergeStrategy(opts.defaltOp)),
			netconf.WithTestStrategy(netconf.TestStrategy(opts.testOp)),
		); err != nil {
			device.Log.Errorf("Failed to edit candidate config: %v", err)
			return err
		}

		if validate {
			device.Log.Debugf("Validating %s datastore", datastore)
			if err := session.Validate(ctx, datastore); err != nil {
				return err
			}
		}

		device.Log.Debug("Committing changes")
		if err := session.Commit(ctx); err != nil {
			return err
		}

		device.Log.Debugf("Unlocking %s datastore", datastore)
		if err := session.Unlock(ctx, datastore); err != nil {
			return err
		}
	}
	device.Log.Infof("Executed %d edit-config requests, took %.3f seconds", len(files), time.Since(start).Seconds())

	start = time.Now()
	if opts.copy && startup && netconf.TestStrategy(opts.testOp) != netconf.TestOnly {
		if err := session.CopyConfig(ctx, netconf.Running, netconf.Startup); err != nil {
			return err
		}
		device.Log.Infof("Executed copy-config request, took %.3f seconds", time.Since(start).Seconds())
	}
	return nil
}
