package ssh

import (
	"time"

	"golang.org/x/crypto/ssh"
)

const keepAliveName = "NETCONF_SUBSCRIPTION_KEEPALIVE"

func keepAlive(conn *ssh.Client) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if _, _, err := conn.SendRequest(keepAliveName, true, nil); err != nil {
			return
		}
	}
}
