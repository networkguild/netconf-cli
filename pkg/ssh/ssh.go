package ssh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mikkeloscar/sshconfig"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"golang.org/x/crypto/ssh"
)

type Client struct {
	DeviceSSHClient *ssh.Client
	tunnelSSHClient *ssh.Client
	logger          *log.Logger
}

func (c *Client) Close() error {
	c.logger.Info("Closing all underlying leftover ssh connections")
	if c.tunnelSSHClient != nil {
		return errors.Join(c.DeviceSSHClient.Close(), c.tunnelSSHClient.Close())
	} else {
		return c.DeviceSSHClient.Close()
	}
}

func DialSSH(device *config.Device, keepalive bool) (*Client, error) {
	tunnelSSHClient, deviceSSHConfig, err := parseConnection(device)
	if err != nil || deviceSSHConfig == nil {
		return nil, err
	}

	deviceAddr := fmt.Sprintf("%s:%d", device.IP, device.Port)
	if tunnelSSHClient != nil {
		if keepalive {
			go keepAlive(tunnelSSHClient)
		}

		conn, err := tunnelSSHClient.Dial("tcp", deviceAddr)
		if err != nil {
			return nil, errors.Join(err, tunnelSSHClient.Close())
		}

		device.Log.Debugf("Connecting to device %s through proxy", deviceAddr)
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, deviceAddr, deviceSSHConfig)
		if err != nil {
			return nil, errors.Join(err, tunnelSSHClient.Close())
		}
		device.Log.Infof("Connected to device %s through proxy", deviceAddr)

		sshClient := ssh.NewClient(sshConn, chans, reqs)
		if keepalive {
			go keepAlive(sshClient)
		}
		return &Client{
			DeviceSSHClient: sshClient,
			tunnelSSHClient: tunnelSSHClient,
			logger:          device.Log,
		}, nil

	} else {
		device.Log.Debugf("Connecting to device %s", deviceAddr)
		sshClient, err := ssh.Dial("tcp", deviceAddr, deviceSSHConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to dial to host: %s, %v", deviceAddr, err)
		}
		device.Log.Infof("Connected to device %s", deviceAddr)

		return &Client{
			DeviceSSHClient: sshClient,
			logger:          device.Log,
		}, nil
	}
}

func parseConnection(device *config.Device) (*ssh.Client, *ssh.ClientConfig, error) {
	hosts, err := readUserSSHConfig()
	if err != nil {
		log.Warnf("Failed to find ssh config file, using input configs")
		return nil, &ssh.ClientConfig{
			User:            device.Username,
			Auth:            []ssh.AuthMethod{ssh.Password(device.Password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}, nil
	}

	var (
		tunnelSSHClient *ssh.Client
		deviceSSHConfig *ssh.ClientConfig
	)
	for _, host := range hosts {
		match, err := regexp.MatchString(host.Host[0], device.IP)
		if err != nil {
			continue
		}
		if match {
			if host.ProxyCommand != "" {
				if !strings.ContainsAny(host.ProxyCommand, "-W") {
					return nil, nil, fmt.Errorf("only proxy command with -W is supported, got: %s", host.ProxyCommand)
				}

				// assuming that ProxyCommand is in format `ProxyCommand ssh -W %h:%p proxy` or `ssh proxy -W %h:%p`
				proxyCommand := strings.Split(host.ProxyCommand, " ")
				jumpHost, err := getJumpHostConfigs(proxyCommand[1:], hosts)
				if err != nil {
					return nil, nil, err
				}

				device.Log.Debugf("Connecting to proxy %s", jumpHost.hostname)
				tunnelClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", jumpHost.hostname, jumpHost.port), jumpHost.sshCfg)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to dial to tunnel host: %s, %v", jumpHost.hostname, err)
				}
				device.Log.Infof("Connected to proxy %s", jumpHost.hostname)

				tunnelSSHClient = tunnelClient
			}

			var auth ssh.AuthMethod
			if host.IdentityFile != "" {
				signer, err := parseSSHSigner(host.IdentityFile)
				if err != nil {
					return nil, nil, err
				}
				auth = ssh.PublicKeys(signer)
			} else {
				auth = ssh.Password(device.Password)
			}

			var user string
			if host.User != "" {
				user = host.User
			} else {
				user = device.Username
			}
			deviceSSHConfig = &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{auth},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
			break
		}
	}

	if deviceSSHConfig == nil {
		deviceSSHConfig = &ssh.ClientConfig{
			User:            device.Username,
			Auth:            []ssh.AuthMethod{ssh.Password(device.Password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}
	return tunnelSSHClient, deviceSSHConfig, nil
}

func readUserSSHConfig() ([]*sshconfig.SSHHost, error) {
	path := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	hosts, err := sshconfig.Parse(path)
	if err != nil {
		fallbackConfig := "/etc/ssh/ssh_config"
		log.Warnf("failed to find ssh config from user home dir, trying fallback: %s", fallbackConfig)
		return sshconfig.Parse(fallbackConfig)
	}
	return hosts, nil
}

type jumpHostConfig struct {
	hostname string
	port     int
	sshCfg   *ssh.ClientConfig
}

func getJumpHostConfigs(proxyCommand []string, hosts []*sshconfig.SSHHost) (*jumpHostConfig, error) {
	var name string
	idx := slices.Index(proxyCommand, "-W")
	if length := len(proxyCommand); idx == length-1 {
		name = proxyCommand[idx-1]
	} else {
		name = proxyCommand[length-1]
	}

	for _, host := range hosts {
		if slices.Contains(host.Host, name) {
			if host.IdentityFile == "" {
				return nil, fmt.Errorf("identity file is required for jump host")
			}

			signer, err := parseSSHSigner(host.IdentityFile)
			if err != nil {
				return nil, err
			}

			var port int
			if host.Port == 0 {
				port = 22
			} else {
				port = host.Port
			}

			return &jumpHostConfig{
				hostname: host.HostName,
				port:     port,
				sshCfg: &ssh.ClientConfig{
					User:            host.User,
					Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				},
			}, nil
		}
	}
	return nil, fmt.Errorf("failed to find configs for jump host: %s", name)
}
