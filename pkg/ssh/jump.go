package ssh

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mikkeloscar/sshconfig"
	"github.com/networkguild/netconf-cli/pkg/config"
	"golang.org/x/crypto/ssh"
)

const defaultSSHIdentityFile = "~/.ssh/id_rsa"

type jumpConfig struct {
	address string
	port    int
	sshCfg  *ssh.ClientConfig
}

func (c *Client) dialJumpHost(command string, device *config.Device) (*ssh.Client, error) {
	// assuming that ProxyCommand is in format `ProxyCommand ssh -W %h:%p proxy` or `ssh proxy -W %h:%p`
	proxyCommand := strings.Split(command, " ")
	sshConfigProxy := parseJumpHost(proxyCommand[1:], c.sshConfig)

	if !c.multiplexing {
		jump, err := c.getJumpHostConfigs(sshConfigProxy)
		if err != nil {
			return nil, err
		}
		device.Log.Debugf("Connecting to proxy %s", jump.address)
		uniqueConn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", jump.address, jump.port), jump.sshCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to dial tunnel host: %s, %v", jump.address, err)
		}
		device.Log.Infof("Connected to proxy %s", jump.address)
		if c.keepalive {
			go keepAlive(uniqueConn)
		}
		c.proxies.Set(device.IP, uniqueConn)
		return uniqueConn, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	conn, found := c.proxies.Get(sshConfigProxy.HostName)
	if !found {
		jump, err := c.getJumpHostConfigs(sshConfigProxy)
		if err != nil {
			return nil, err
		}
		log.Debugf("Connecting to proxy %s", jump.address)
		conn, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", jump.address, jump.port), jump.sshCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to dial tunnel host: %s, %v", jump.address, err)
		}
		log.Infof("Connected to proxy %s", jump.address)
		if c.keepalive {
			go keepAlive(conn)
		}
		c.proxies.Set(sshConfigProxy.HostName, conn)
	}
	return conn, nil
}

const defaultSSHPort = 22

func parseJumpHost(proxyCommand []string, hosts []*sshconfig.SSHHost) *sshconfig.SSHHost {
	var name, user string
	idx := slices.Index(proxyCommand, "-W")
	if length := len(proxyCommand); idx == 1 {
		name = proxyCommand[idx-1]
	} else {
		name = proxyCommand[length-1]
	}

	nameWithUser := strings.Split(name, "@")
	if len(nameWithUser) == 2 {
		user, name = nameWithUser[0], nameWithUser[1]
	}

	for _, host := range hosts {
		if slices.Contains(host.Host, name) {
			if user != "" {
				host.User = user
			}
			if host.Port == 0 {
				host.Port = defaultSSHPort
			}
			return host
		}
	}
	return nil
}

func (c *Client) getJumpHostConfigs(host *sshconfig.SSHHost) (jumpConfig, error) {
	if len(c.signers) == 0 {
		identityDefault := os.Getenv("SSH_DEFAULT_IDENTITY_FILE")
		if host.IdentityFile == "" && identityDefault != "" {
			host.IdentityFile = identityDefault
		} else {
			host.IdentityFile = defaultSSHIdentityFile
		}
		log.Warnf("no identity file found for host: %s, using default: %s", host.Host[0], host.IdentityFile)

		signer, err := parseSSHSigner(host.IdentityFile)
		if err != nil {
			return jumpConfig{}, err
		}
		c.signers = append(c.signers, signer)
	}

	if host.HostName == "" {
		return jumpConfig{}, fmt.Errorf("address is required for jump host")
	}
	return jumpConfig{
		address: host.HostName,
		port:    host.Port,
		sshCfg: &ssh.ClientConfig{
			User:            host.User,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(c.signers...)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         10 * time.Second,
		},
	}, nil
}
