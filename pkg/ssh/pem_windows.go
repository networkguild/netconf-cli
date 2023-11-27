//go:build windows

package ssh

import (
	"errors"
	"os"
	"sync"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var pemStore sync.Map

func parseSSHSigner(file string) (ssh.Signer, error) {
	expandedPath, err := homedir.Expand(file)
	if err != nil {
		return nil, err
	}

	if sig, ok := pemStore.Load(expandedPath); ok {
		return *sig.(*ssh.Signer), nil
	}

	key, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, err
	}

	var sign ssh.Signer
	if signer, err := ssh.ParsePrivateKey(key); err != nil {
		var e *ssh.PassphraseMissingError
		if errors.As(err, &e) {
			log.Info("Enter password for private key:")
			password, err := term.ReadPassword(syscall.STD_INPUT_HANDLE)
			if err != nil {
				return nil, err
			}

			if s, err := ssh.ParsePrivateKeyWithPassphrase(key, password); err != nil {
				return nil, err
			} else {
				sign = s
			}
		}
	} else {
		sign = signer
	}
	pemStore.Store(expandedPath, &sign)
	return sign, nil
}
