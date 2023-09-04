package config

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/utils"
)

type Config struct {
	Devices []Device
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

	if ip := viper.GetString("host"); ip != "" {
		devices = append(devices, Device{
			IP:       ip,
			Username: username,
			Password: password,
			Port:     port,
			Ctx:      ctx,
			Log:      copyDefaultLogger(ip),
		})
	} else {
		hosts, err := utils.ReadInventoryFromUser(viper.GetString("inventory"))
		if err != nil {
			return nil, fmt.Errorf("either --host or --invertory, -i must be specified")
		}

		for _, host := range hosts {
			if host.IP == "" {
				continue
			}
			devices = append(devices, Device{
				IP:       host.IP,
				Username: username,
				Password: password,
				Port:     port,
				Suffix:   host.Suffix,
				Ctx:      ctx,
				Log:      copyDefaultLogger(host.IP),
			})
		}
	}

	return &Config{
		Devices: devices,
	}, nil
}

func copyDefaultLogger(ip string) *log.Logger {
	logger := log.With()
	logger.SetPrefix(ip)
	return logger
}
