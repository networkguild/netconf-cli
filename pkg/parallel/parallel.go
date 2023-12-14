package parallel

import (
	"encoding/xml"
	"errors"
	"fmt"
	"runtime"

	"github.com/alphadose/haxmap"
	"github.com/charmbracelet/log"
	"github.com/networkguild/netconf"
	"github.com/networkguild/netconf-cli/pkg/config"
	"github.com/networkguild/netconf-cli/pkg/ssh"
	ncssh "github.com/networkguild/netconf/transport/ssh"
	"golang.org/x/sync/errgroup"
)

var errorStore *haxmap.Map[string, error]

func RunParallel(config *config.Config, f func(device *config.Device, session *netconf.Session) error) error {
	errorStore = haxmap.New[string, error](uintptr(len(config.Devices)))
	devicesCount := len(config.Devices)

	var wg errgroup.Group
	wg.SetLimit(runtime.GOMAXPROCS(0))

	client := ssh.NewClient(devicesCount, config.Multiplexing, false)
	defer client.Close()

	for _, device := range config.Devices {
		d := device
		wg.Go(func() error {
			sshClient, err := client.DialSSH(&d)
			if err != nil {
				errorStore.Set(d.IP, err)
				return err
			}
			defer client.CloseDeviceConn(d.IP)

			transport, err := ncssh.NewTransport(sshClient)
			if err != nil {
				errorStore.Set(d.IP, err)
				return err
			}
			defer transport.Close()

			session, err := netconf.Open(transport, netconf.WithLogger(d.Log))
			if err != nil {
				errorStore.Set(d.IP, err)
				return fmt.Errorf("failed to exchange hello messages, error: %v", err)
			}
			defer session.Close(d.Ctx)

			d.Log.Debugf("Started netconf session with id: %d", session.SessionID())
			if err := f(&d, session); err != nil {
				errorStore.Set(d.IP, err)
				return err
			}
			return nil
		})
	}

	defer func() {
		errorStore.ForEach(func(ip string, err error) bool {
			var (
				rpcErr netconf.RPCError
				msg    = fmt.Sprintf("Device %s failed", ip)
			)
			if errors.As(err, &rpcErr) {
				if xmlErr, err := xml.MarshalIndent(&rpcErr, "", "  "); err == nil {
					log.Error(msg, "RPCError", string(xmlErr))
					return true
				}
			}
			log.Error(msg, "error", err)
			return true
		})
	}()
	return wg.Wait()
}
