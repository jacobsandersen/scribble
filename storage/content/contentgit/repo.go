package contentgit

import (
	"fmt"

	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/indieinfra/scribble/config"
)

func BuildGitAuth(cfg *config.GitContentStrategy) (transport.AuthMethod, error) {
	switch cfg.Auth.Method {
	case "plain":
		return &http.BasicAuth{
			Username: cfg.Auth.Plain.Username,
			Password: cfg.Auth.Plain.Password,
		}, nil
	case "ssh":
		pubkeys, err := ssh.NewPublicKeysFromFile(cfg.Auth.Ssh.Username, cfg.Auth.Ssh.PrivateKeyFilePath, cfg.Auth.Ssh.Passphrase)

		if err != nil {
			return nil, fmt.Errorf("failed to prepare content git ssh authentication: %w", err)
		}

		return pubkeys, nil
	default:
		return nil, fmt.Errorf("invalid git authentication method %v", cfg.Auth.Method)
	}
}
