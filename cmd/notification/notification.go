package notification

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/networkguild/netconf"
	ncssh "github.com/networkguild/netconf/transport/ssh"
	"github.com/spf13/cobra"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/ssh"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/utils"
)

var opts struct {
	getStreams bool
	persist    bool
	stream     string
	duration   time.Duration
}

func NewNotificationCommand() *cobra.Command {
	notificationCmd := &cobra.Command{
		Use:   "notification",
		Short: "Execute create-subscription rpc",
		Long: `Execute create-subscription rpc for initiating an event notification subscription that will send asynchronous notifications.

If you want to save notifications to file, use --save flag. Default file name is ip + notifications.xml or provide file name via inventory file

# get all available streams from device
netconf notification --host 192.168.1.1 --get

# subscribe to notification stream, cancel with ctrl+c
netconf notification --host 192.168.1.1 --stream NETCONF

# subscribe to notification stream with duration, cancel with ctrl+c
netconf notification --host 192.168.1.1 --stream NETCONF --duration 12m30s`,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				sig := <-sigs
				log.Warnf("Received signal %s, exiting all notification subscriptions...", sig)
				cancel()
			}()

			if opts.duration != 0 {
				// monitor when subscription ends, as server does not close session
				go func() {
					time.Sleep(opts.duration + 5*time.Second)
					sigs <- syscall.SIGTERM
				}()
			}

			cfg, err := config.ParseConfig(ctx)
			if err != nil {
				log.Fatalf("Failed to init config, error: %v", err)
			}

			runSubscriptions(cfg.Devices)
		},
	}
	flags := notificationCmd.Flags()
	flags.BoolVar(&opts.getStreams, "get", false, "get available notification streams")
	flags.StringVarP(&opts.stream, "stream", "s", "NETCONF", "stream to subscribe")
	flags.DurationVarP(&opts.duration, "duration", "d", 0, "duration for subscription, eg. 2h30m45s")
	flags.BoolVar(&opts.persist, "save", false, "save notifications to file, default name is used, if no suffix provided")

	return notificationCmd
}

const subscriptionGet = `<netconf xmlns="urn:ietf:params:xml:ns:netmod:notification"><streams/></netconf>`

func runSubscriptions(devices []config.Device) {
	var wg sync.WaitGroup

	wg.Add(len(devices))
	for _, device := range devices {
		d := device
		handler := func(notification netconf.Notification) {
			var xmlString string
			notif, err := xml.Marshal(&notification)
			if err != nil {
				xmlString = notification.String()
			} else {
				xmlString = string(notif)
			}
			xmlString = utils.FormatXML(xmlString)
			d.Log.Infof("Received notification, timestamp: %s", notification.EventTime)

			if opts.persist {
				var fileName string
				if d.Suffix != "" {
					fileName = fmt.Sprintf("%s-%s", d.IP, d.Suffix)
				} else {
					fileName = fmt.Sprintf("%s-notifications.xml", d.IP)
				}
				file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					d.Log.Warnf("Failed to open file for writing")
					return
				}
				defer file.Close()

				file.WriteString(xmlString)
			} else {
				d.Log.Infof("Notification:%s", xmlString)
			}
		}

		go func() {
			defer wg.Done()
			start := time.Now()
			client, err := ssh.DialSSH(&d, true)
			if err != nil {
				log.Errorf("failed to dial ssh, ip: %s, error: %v", d.IP, err)
				return
			}
			defer client.Close()

			transport, err := ncssh.NewTransport(client.DeviceSSHClient)
			if err != nil {
				log.Errorf("failed to create new transport error: %v", err)
				return
			}
			defer transport.Close()

			session, err := netconf.Open(transport, netconf.WithNotificationHandler(handler))
			if err != nil {
				log.Errorf("failed to exchange hello messages, ip: %s, error: %v", d.IP, err)
				return
			}
			defer session.Close(d.Ctx)

			if opts.getStreams {
				get, err := session.Get(d.Ctx,
					netconf.WithSubtreeFilter(subscriptionGet),
				)
				if err != nil {
					log.Errorf("Failed to get available streams: %v", err)
				}
				xmlString := utils.FormatXML(get.String())
				d.Log.Infof("Available streams:\n%s", xmlString)
				d.Log.Infof("Fetched available notifications streams, took %.3f seconds", time.Since(start).Seconds())
			} else {
				if opts.duration != 0 {
					if err := session.CreateSubscription(d.Ctx,
						netconf.WithStreamOption(opts.stream),
						netconf.WithStartTimeOption(start),
						netconf.WithStopTimeOption(start.Add(opts.duration)),
					); err != nil {
						d.Log.Errorf("Failed to create subscription with duration: %s, %v", err, opts.duration)
						return
					}
					d.Log.Infof("Created subscription with duration: %s, took %.3f seconds", opts.duration, time.Since(start).Seconds())
				} else {
					if err := session.CreateSubscription(d.Ctx, netconf.WithStreamOption(opts.stream)); err != nil {
						d.Log.Errorf("Failed to create subscription: %v", err)
						return
					}
					d.Log.Infof("Created subscription, took %.3f seconds", time.Since(start).Seconds())
				}
				<-d.Ctx.Done()

				d.Log.Infof("Subscription %s ended, duration %.3f seconds", opts.stream, time.Since(start).Seconds())
			}
		}()
	}
	wg.Wait()
}
