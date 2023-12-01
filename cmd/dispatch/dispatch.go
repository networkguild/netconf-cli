package dispatch

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
	useLock bool
	file    string
}

var files [][]byte

func NewDispatchCommand() *cobra.Command {
	dispatchCmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Execute rpc",
		Long: `Execute user-defined rpc with optional options
Use --debug flag, to log all dispatch replies (Recommended)

# dispatch
netconf dispatch --host 192.168.1.1 --file dispatch.xml`,
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

			if err := parallel.RunParallel(cfg.Devices, runDispatch); err != nil {
				log.Fatalf("Failed to execute dispatch")
			}
		},
	}
	flags := dispatchCmd.Flags()
	flags.StringVarP(&opts.file, "file", "f", "", "stdin, file or directory containing xml files")
	flags.BoolVarP(&opts.useLock, "lock", "l", false, "run with datastore lock")

	return dispatchCmd
}

func runDispatch(device *config.Device, session *netconf.Session) error {
	ctx, cancel := context.WithTimeout(device.Ctx, 5*time.Minute)
	defer cancel()

	var (
		capabilities = session.ServerCapabilities()
		candidate    = slices.Contains(capabilities, netconf.CandidateCapability)
	)
	datastore := netconf.Running
	if candidate {
		datastore = netconf.Candidate
	}

	start := time.Now()
	for _, data := range files {
		if opts.useLock {
			device.Log.Debugf("Locking %s datastore", datastore)
			if err := session.Lock(ctx, datastore); err != nil {
				return err
			}

			if reply, err := session.Dispatch(ctx, data); err != nil {
				return err
			} else {
				replyString := utils.FormatXML(reply.String())
				device.Log.Debugf("Dispatch reply:\n%s", replyString)
			}

			device.Log.Debug("Committing changes")
			if err := session.Commit(ctx); err != nil {
				return err
			}

			device.Log.Debugf("Unlocking %s datastore", datastore)
			if err := session.Unlock(ctx, datastore); err != nil {
				return err
			}
		} else {
			if reply, err := session.Dispatch(ctx, data); err != nil {
				return err
			} else {
				replyString := utils.FormatXML(reply.String())
				device.Log.Debugf("Dispatch reply:\n%s", replyString)
			}
		}
	}
	device.Log.Infof("Executed %d dispatch requests, took %.3f seconds", len(files), time.Since(start).Seconds())
	return nil
}
