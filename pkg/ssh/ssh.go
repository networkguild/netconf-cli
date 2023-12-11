package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/charmbracelet/log"
	"github.com/mikkeloscar/sshconfig"
	"github.devcloud.elisa.fi/netops/netconf-go/pkg/config"
	"golang.org/x/crypto/ssh"
)

type Client struct {
	devices *haxmap.Map[string, deviceConn]
	proxies *haxmap.Map[string, *ssh.Client]

	signers []ssh.Signer
	agent   net.Conn

	sshConfig []*sshconfig.SSHHost
	lock      sync.Mutex

	multiplexing bool
	keepalive    bool
}

type deviceConn struct {
	conn   *ssh.Client
	logger *log.Logger
}

func NewClient(devicesCount int, multiplexing, keepalive bool) *Client {
	client := Client{
		devices:      haxmap.New[string, deviceConn](uintptr(devicesCount)),
		proxies:      haxmap.New[string, *ssh.Client](),
		multiplexing: multiplexing,
		keepalive:    keepalive,
	}

	var err error
	client.sshConfig, err = readUserSSHConfig(filepath.Join(os.Getenv("HOME"), ".ssh", "config"))
	if err != nil {
		log.Warnf("Failed to find ssh config file, using input configs")
	}
	return &client
}

func (c *Client) DialSSH(device *config.Device) (*ssh.Client, error) {
	_, found := c.devices.Get(device.IP)
	if found {
		return nil, fmt.Errorf("device %s is already connected", device.IP)
	}
	sshClient, err := c.parseConnection(device)
	if err != nil {
		return nil, err
	}
	return sshClient, nil
}

func (c *Client) Close() error {
	log.Info("Closing all underlying leftover ssh connections")
	c.devices.ForEach(func(ip string, conn deviceConn) bool {
		if err := conn.conn.Close(); err != nil {
			conn.logger.Warnf("failed to close device connection: %v", err)
		}
		return true
	})
	c.proxies.ForEach(func(ip string, conn *ssh.Client) bool {
		if err := conn.Close(); err != nil {
			log.Warnf("failed to close proxy connection: %v", err)
		}
		return true
	})
	if c.agent != nil {
		if err := c.agent.Close(); err != nil {
			log.Warnf("failed to close ssh-agent connection: %v", err)
		}
	}
	return nil
}

func (c *Client) CloseDeviceConn(ip string) error {
	if !c.multiplexing {
		if proxy, found := c.proxies.GetAndDel(ip); found {
			defer proxy.Close()
		}
	}
	device, found := c.devices.GetAndDel(ip)
	if !found {
		return fmt.Errorf("failed to find existing device connection for %s", ip)
	}
	device.logger.Debug("Closing device ssh connections")
	return device.conn.Close()
}

func (c *Client) parseConnection(device *config.Device) (*ssh.Client, error) {
	deviceAddr := fmt.Sprintf("%s:%d", device.IP, device.Port)
	for _, host := range c.sshConfig {
		match, err := regexp.MatchString(host.Host[0], device.IP)
		if err != nil {
			continue
		}
		if match {
			var useProxy bool
			var proxyConn *ssh.Client
			if host.ProxyCommand != "" {
				if !strings.ContainsAny(host.ProxyCommand, "-W") {
					return nil, fmt.Errorf("only proxy command with -W is supported, got: %s", host.ProxyCommand)
				}

				if proxyConn, err = c.dialJumpHost(host.ProxyCommand, device); err != nil {
					return nil, err
				}
				useProxy = true
			}

			var auth ssh.AuthMethod
			if host.IdentityFile != "" {
				if len(c.signers) == 0 {
					signer, err := parseSSHSigner(host.IdentityFile)
					if err != nil {
						return nil, err
					}
					c.signers = append(c.signers, signer)
				}
				auth = ssh.PublicKeys(c.signers...)
			} else {
				auth = ssh.Password(device.Password)
			}

			user := host.User
			if user == "" {
				user = device.Username
			}

			deviceConf := &ssh.ClientConfig{
				User:            user,
				Auth:            []ssh.AuthMethod{auth},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				Timeout:         10 * time.Second,
			}

			if useProxy {
				conn, err := proxyConn.Dial("tcp", deviceAddr)
				if err != nil {
					return nil, errors.Join(err, proxyConn.Close())
				}

				device.Log.Debugf("Connecting to device %s through proxy", deviceAddr)
				sshConn, chans, reqs, err := ssh.NewClientConn(conn, deviceAddr, deviceConf)
				if err != nil {
					return nil, errors.Join(err, proxyConn.Close())
				}
				device.Log.Infof("Connected to device %s through proxy", deviceAddr)

				sshClient := ssh.NewClient(sshConn, chans, reqs)
				if c.keepalive {
					go keepAlive(sshClient)
				}
				c.devices.Set(device.IP, deviceConn{conn: sshClient, logger: device.Log})
				return sshClient, nil
			} else {
				device.Log.Debugf("Connecting to device %s", deviceAddr)
				sshClient, err := ssh.Dial("tcp", deviceAddr, deviceConf)
				if err != nil {
					return nil, fmt.Errorf("failed to dial to host: %s, %v", deviceAddr, err)
				}
				device.Log.Infof("Connected to device %s", deviceAddr)
				if c.keepalive {
					go keepAlive(sshClient)
				}
				c.devices.Set(device.IP, deviceConn{conn: sshClient, logger: device.Log})
				return sshClient, nil
			}
		}
	}

	defaultConfig := &ssh.ClientConfig{
		User:            device.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(device.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	device.Log.Debugf("Connecting to device %s", deviceAddr)
	sshClient, err := ssh.Dial("tcp", deviceAddr, defaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to host: %s, %v", deviceAddr, err)
	}
	device.Log.Infof("Connected to device %s", deviceAddr)
	c.devices.Set(device.IP, deviceConn{conn: sshClient, logger: device.Log})
	if c.keepalive {
		go keepAlive(sshClient)
	}
	return sshClient, nil
}

func readUserSSHConfig(path string) ([]*sshconfig.SSHHost, error) {
	hosts, err := sshconfig.Parse(path)
	if err != nil {
		fallbackConfig := "/etc/ssh/ssh_config"
		log.Warnf("failed to find ssh config from user home dir, trying fallback: %s", fallbackConfig)
		return sshconfig.Parse(fallbackConfig)
	}
	return hosts, nil
}
