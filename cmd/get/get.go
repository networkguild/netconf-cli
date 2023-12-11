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

var opts struct {
	filters  string
	defaults string
	persist  bool
}

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
<state xmlns="urn:nokia.com:sros:ns:yang:sr:state">
        <port><ethernet><lldp><dest-mac>
            <remote-system/>
        </dest-mac></lldp></ethernet></port>
</state>
<configure xmlns="urn:nokia.com:sros:ns:yang:sr:conf">
        <service><vprn/></service>
</configure>

All filters are fetched same time.`,
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

			if err := parallel.RunParallel(cfg, runGet); err != nil {
				log.Fatalf("Failed to execute get")
			}
		},
	}
	flags := getCmd.Flags()
	flags.StringVarP(&opts.filters, "filter", "f", "", "filter option, stdin or file containing filters")
	flags.StringVarP(&opts.defaults, "with-defaults", "d", "", "with-defaults option, report-all|report-all-tagged|trim|explicit")
	flags.BoolVar(&opts.persist, "save", false, "save output to file, default name is used, if no suffix provided")

	return getCmd
}

func runGet(device *config.Device, session *netconf.Session) error {
	ctx, cancel := context.WithTimeout(device.Ctx, 5*time.Minute)
	defer cancel()

	start := time.Now()
	reply, err := session.Get(ctx,
		netconf.WithDefaultMode(netconf.DefaultsMode(opts.defaults)),
		netconf.WithSubtreeFilter(opts.filters),
	)
	if err != nil {
		device.Log.Errorf("Failed to get subtree: %v", err)
		return err
	}

	replyString := utils.FormatXML(reply.String())
	if opts.persist {
		var name string
		if device.Suffix != "" {
			name = fmt.Sprintf("%s-%s", device.IP, device.Suffix)
		} else {
			name = fmt.Sprintf("%s-get-filters.xml", device.IP)
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
		device.Log.Infof("Get reply:\n%s", replyString)
	}
	device.Log.Infof("Executed get filter request, took %.3f seconds", time.Since(start).Seconds())
	return nil
}
