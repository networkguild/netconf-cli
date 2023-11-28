package get_config

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/networkguild/netconf"
	"github.com/spf13/cobra"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/parallel"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/utils"
)

var opts struct {
	filters  string
	defaults string
	persist  bool
	source   string
}

func NewGetConfigCommand() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get-config",
		Short: "Execute get-config rpc",
		Long: `Execute get-config rpc for retrieving configuration from device datastore."

# get-config using inventory host file, using startup datastore, writes to file
netconf get-config --inventory hosts.ini --save --source startup

# get-config using host flag, prints to stdout
netconf get-config --host 192.168.1.1 --password pass --username user`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.ParseConfig(context.Background())
			if err != nil {
				log.Fatalf("Failed to init config, error: %v", err)
			}

			if opts.filters != "" {
				opts.filters, err = utils.ReadFiltersFromUser(opts.filters)
				if err != nil {
					log.Fatalf("Failed to read filters, error: %v", err)
				}
			}

			if err := parallel.RunParallel(cfg.Devices, runGetConfig); err != nil {
				log.Fatalf("Failed to execute get-config")
			}
		},
	}
	flags := getCmd.Flags()
	flags.StringVarP(&opts.filters, "filter", "f", "", "filter option, stdin or file containing filters")
	flags.StringVarP(&opts.defaults, "with-defaults", "d", "", "with-defaults option, report-all|report-all-tagged|trim|explicit")
	flags.BoolVar(&opts.persist, "save", false, "save output to file, default name is used, if no suffix provided")
	flags.StringVarP(&opts.source, "source", "s", "running", "running|candidate|startup")

	return getCmd
}

func runGetConfig(device *config.Device, session *netconf.Session) error {
	ctx, cancel := context.WithTimeout(device.Ctx, 5*time.Minute)
	defer cancel()

	start := time.Now()
	reply, err := session.GetConfig(ctx,
		netconf.Datastore(opts.source),
		netconf.WithDefaultMode(netconf.DefaultsMode(opts.defaults)),
		netconf.WithSubtreeFilter(opts.filters),
	)
	if err != nil {
		return fmt.Errorf("failed to get %s config, ip: %s, error: %v", opts.source, device.IP, err)
	}

	replyString := utils.FormatXML(reply.String())
	if opts.persist {
		var name string
		if device.Suffix != "" {
			name = fmt.Sprintf("%s-%s", device.IP, device.Suffix)
		} else {
			name = fmt.Sprintf("%s-get-config-%s.xml", device.IP, utils.TimeStamp())
		}

		file, err := os.Create(name)
		if err != nil {
			device.Log.Errorf("Failed to create file: %v", err)
		} else {
			defer file.Close()

			file.WriteString(replyString)
			device.Log.Infof("Saved get reply to file %s", name)
		}
	} else {
		device.Log.Infof("Get-config reply:\n%s", replyString)
	}
	device.Log.Infof("Executed get-config request, took %.3f seconds", time.Since(start).Seconds())
	return nil
}
