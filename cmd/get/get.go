package get

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

var (
	filterFile string
	defaults   string
	persist    bool
	filters    []byte
)

func NewGetCommand() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Execute get rpc",
		Long: `Execute get rpc for retrieving configuration and state information from device datastore.

# get using env values and stdin filter, writes to file
echo "<devm xmlns=\"urn:huawei:yang:huawei-devm\"></devm>" | netconf get --inventory hosts.ini --save

# get using env values and filter file, prints to stdout
netconf get --host 192.168.1.1 --filters filters.xml --debug

Example filters file content:
# filters.xml 
<devm xmlns="urn:huawei:yang:huawei-devm"/>
<ifm xmlns="urn:huawei:yang:huawei-ifm"/>

One subtree/filter per line, no maximum amount of filters. All filters are fetched same time.`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.ParseConfig(context.Background())
			if err != nil {
				log.Fatalf("Failed to init config, error: %v", err)
			}

			userFilters, err := utils.ReadFiltersFromUser(filterFile)
			if err != nil {
				log.Fatalf("Failed to read filters, error: %v", err)
			}

			filters = userFilters
			if err := parallel.RunParallel(cfg.Devices, runGet); err != nil {
				log.Fatalf("Failed to run netconf, error: %v", err)
			}
		},
	}
	flags := getCmd.Flags()
	flags.StringVarP(&filterFile, "filter", "f", "", "filter option, stdin or file containing filters")
	flags.StringVarP(&defaults, "with-defaults", "d", "", "with-defaults option, report-all|report-all-tagged|trim|explicit")
	flags.BoolVar(&persist, "save", false, "save output to file, default name is used, if no suffix provided")

	return getCmd
}

func runGet(device *config.Device, session *netconf.Session) error {
	ctx, cancel := context.WithTimeout(device.Ctx, 5*time.Minute)
	defer cancel()

	start := time.Now()
	reply, err := session.Get(ctx,
		netconf.WithDefaultMode(netconf.DefaultsMode(defaults)),
		netconf.WithFilter(netconf.Filter(filters)),
	)
	if err != nil {
		device.Log.Errorf("Failed to get subtree: %v", err)
		return err
	}

	if persist {
		var name string
		if device.Suffix != "" {
			name = fmt.Sprintf("%s-%s", device.IP, device.Suffix)
		} else {
			name = fmt.Sprintf("%session-get-filters.xml", device.IP)
		}

		file, err := os.Create(name)
		if err != nil {
			device.Log.Errorf("Failed to create file: %v, printing reply", err)
			device.Log.Infof("Get reply:\n%s", reply.Raw())
		} else {
			defer file.Close()

			_, err = file.Write(reply.Raw())
			if err != nil {
				return err
			}

			device.Log.Infof("Saved get reply to file %s", name)
		}
	} else {
		device.Log.Infof("Get reply:\n%s", reply.Raw())

	}
	device.Log.Infof("Executed get filter request, took %.3f seconds", time.Since(start).Seconds())
	return nil
}
