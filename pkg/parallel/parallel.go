package parallel

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/networkguild/netconf"
	ncssh "github.com/networkguild/netconf/transport/ssh"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/ssh"
	"golang.org/x/sync/errgroup"
)

// map[ip]error
var errorStore sync.Map

func RunParallel(devices []config.Device, f func(device *config.Device, session *netconf.Session) error) error {
	var wg errgroup.Group
	wg.SetLimit(runtime.GOMAXPROCS(0))
	for _, device := range devices {
		d := device
		wg.Go(func() error {
			client, err := ssh.DialSSH(&d, false)
			if err != nil {
				return fmt.Errorf("failed to dial ssh, ip: %s, error: %v", device.IP, err)
			}
			defer client.Close()

			transport, err := ncssh.NewTransport(client.DeviceSSHClient)
			if err != nil {
				return err
			}
			defer transport.Close()

			session, err := netconf.Open(transport)
			if err != nil {
				return fmt.Errorf("failed to exchange hello messages, ip: %s, error: %v", d.IP, err)
			}
			defer session.Close(d.Ctx)

			if err := f(&d, session); err != nil {
				errorStore.Store(d.IP, err)
				return err
			}
			return nil
		})
	}
	defer func() {
		errorStore.Range(func(key, value interface{}) bool {
			log.Errorf("Device %s failed with error: %v", key, value)
			return true
		})
	}()
	return wg.Wait()
}
