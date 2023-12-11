package config

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/utils"
)

type Config struct {
	Devices      []Device
	Multiplexing bool
}

type Device struct {
	IP       string
	Username string
	Password string
	Port     int
	Suffix   string
	Ctx      context.Context
	Log      *log.Logger
}

func ParseConfig(ctx context.Context) (*Config, error) {
	var (
		devices  []Device
		username = viper.GetString("username")
		password = viper.GetString("password")
		port     = viper.GetInt("port")
	)

	if ips := viper.GetStringSlice("host"); len(ips) > 0 {
		for _, ip := range ips {
			devices = append(devices, Device{
				IP:       ip,
				Username: username,
				Password: password,
				Port:     port,
				Ctx:      ctx,
				Log:      log.WithPrefix(ip),
			})
		}
	} else {
		hosts, err := utils.ReadInventoryFromUser(viper.GetString("inventory"))
		if err != nil {
			return nil, fmt.Errorf("either --host or --invertory, -i must be specified")
		}

		for _, host := range hosts {
			if host.IP == "" {
				continue
			}

			p := port
			if host.Port != 0 {
				p = host.Port
			}
			devices = append(devices, Device{
				IP:       host.IP,
				Username: username,
				Password: password,
				Port:     p,
				Suffix:   host.Suffix,
				Ctx:      ctx,
				Log:      log.WithPrefix(host.IP),
			})
		}
	}

	return &Config{
		Devices:      devices,
		Multiplexing: !viper.GetBool("no-multiplexing"),
	}, nil
}
