package parallel

import (
	"encoding/xml"
	"errors"
	"fmt"
	"runtime"

	"github.com/alphadose/haxmap"
	"github.com/charmbracelet/log"
	"github.com/networkguild/netconf"
	ncssh "github.com/networkguild/netconf/transport/ssh"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/ssh"
	"golang.org/x/sync/errgroup"
)

var errorStore *haxmap.Map[string, error]

func RunParallel(devices []config.Device, f func(device *config.Device, session *netconf.Session) error) error {
	errorStore = haxmap.New[string, error](uintptr(len(devices)))
	var wg errgroup.Group
	wg.SetLimit(runtime.GOMAXPROCS(0))

	for _, device := range devices {
		d := device
		wg.Go(func() error {
			client, err := ssh.DialSSH(&d, false)
			if err != nil {
				errorStore.Set(d.IP, err)
				return err
			}
			defer client.Close()

			transport, err := ncssh.NewTransport(client.DeviceSSHClient)
			if err != nil {
				errorStore.Set(d.IP, err)
				return err
			}
			defer transport.Close()

			session, err := netconf.Open(transport)
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
