package ssh

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testHostIP = "172.30.15.1"

func TestSSHConfigParsing(t *testing.T) {
	tests := []struct {
		name             string
		configPath       string
		expectedJumpUser string
		identityFile     string
	}{
		{
			name:             "read ssh config with user@host format, no identity file",
			configPath:       "testdata/config_no_user",
			expectedJumpUser: "netops-operator-2",
			identityFile:     "~/.ssh/id_ed25519",
		},
		{
			name:             "read ssh config with User and IdentityFile format",
			configPath:       "testdata/config_user",
			expectedJumpUser: "netops-operator",
			identityFile:     "~/.ssh/id_rsa",
		},
	}

	os.Setenv("SSH_DEFAULT_IDENTITY_FILE", "testdata/id_rsa")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hosts, err := readUserSSHConfig(test.configPath)
			if err != nil {
				t.Fatal(err)
			}
			assert.Len(t, hosts, 2)

			for _, host := range hosts {
				match, err := regexp.MatchString(host.Host[0], testHostIP)
				if err != nil {
					t.Fatal(err)
				}

				if match {
					assert.NotEmpty(t, host.ProxyCommand)

					proxyCommand := strings.Split(host.ProxyCommand, " ")
					configs, err := getJumpHostConfigs(proxyCommand[1:], hosts)
					if err != nil {
						t.Fatal(err)
					}

					assert.Equal(t, "10.10.10.10", configs.hostname)
					assert.Equal(t, test.expectedJumpUser, configs.sshCfg.User)
				}
			}
		})
	}
}
