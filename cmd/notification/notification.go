package notification

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-xmlfmt/xmlfmt"
	"github.com/networkguild/netconf"
	ncssh "github.com/networkguild/netconf/transport/ssh"
	"github.com/spf13/cobra"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/ssh"
)

var (
	getStreams bool
	persist    bool
	stream     string
	duration   time.Duration
)

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

			if duration != 0 {
				// monitor when subscription ends, as server does not close session
				go func() {
					select {
					case <-time.After(duration + 5*time.Second):
						sigs <- syscall.SIGTERM
					}
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
	flags.BoolVar(&getStreams, "get", false, "get available notification streams")
	flags.StringVarP(&stream, "stream", "s", "NETCONF", "stream to subscribe")
	flags.DurationVarP(&duration, "duration", "d", 0, "duration for subscription, eg. 2h30m45s")
	flags.BoolVar(&persist, "save", false, "save notifications to file, default name is used, if no suffix provided")

	return notificationCmd
}

const subscriptionGet = `<netconf xmlns="urn:ietf:params:xml:ns:netmod:notification"><streams/></netconf>`

func runSubscriptions(devices []config.Device) {
	var wg sync.WaitGroup

	wg.Add(len(devices))
	for _, device := range devices {
		d := device
		handler := func(notification netconf.Notification) {
			xml := xmlfmt.FormatXML(notification.String(), "", "  ")
			d.Log.Infof("Received notification, timestamp: %s", notification.EventTime)

			if persist {
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

				file.WriteString(xml)
			} else {
				d.Log.Infof("Notification:%s", xml)
			}
		}

		go func() {
			defer wg.Done()
			start := time.Now()
			client, err := ssh.DialSSH(&d, true)
			if err != nil {
				log.Errorf("failed to dial ssh, ip: %s, error: %v", err)
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

			if getStreams {
				get, err := session.Get(d.Ctx,
					netconf.WithFilter(subscriptionGet),
				)
				if err != nil {
					log.Errorf("Failed to get available streams: %v", err)
				}
				d.Log.Infof("Available streams:\n%s", get)
				d.Log.Infof("Fetched available notifications streams, took %.3f seconds", time.Since(start).Seconds())
			} else {
				if duration != 0 {
					_, err := session.CreateSubscription(d.Ctx,
						netconf.WithStreamOption(stream),
						netconf.WithStartTimeOption(start),
						netconf.WithStopTimeOption(start.Add(duration)),
					)
					if err != nil {
						d.Log.Errorf("Failed to create subscription with duration: %s, %v", err, duration)
						return
					}
					d.Log.Infof("Created subscription with duration: %s, took %.3f seconds", duration, time.Since(start).Seconds())
				} else {
					_, err := session.CreateSubscription(d.Ctx, netconf.WithStreamOption(stream))
					if err != nil {
						d.Log.Errorf("Failed to create subscription: %v", err)
						return
					}
					d.Log.Infof("Created subscription, took %.3f seconds", time.Since(start).Seconds())
				}
				<-d.Ctx.Done()

				d.Log.Infof("Subscription %s ended, duration %.3f seconds", stream, time.Since(start).Seconds())
			}
		}()
	}
	wg.Wait()
}
