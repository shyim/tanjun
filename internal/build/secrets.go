package build

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"os"
	"sync"
)

type secretStore struct {
	config       *config.ProjectConfig
	remoteClient *client.Client
	secrets      map[string]string
}

var secretLock = sync.Mutex{}

func (s secretStore) GetSecret(ctx context.Context, secret string) ([]byte, error) {
	if fieldName, ok := s.config.Build.Secrets.FromEnv[secret]; ok {
		if fieldName == "" {
			fieldName = secret
		}

		if val, ok := os.LookupEnv(fieldName); ok {
			return []byte(val), nil
		}

		return nil, fmt.Errorf("could not found value for secret %s: using environment value %s", secret, fieldName)
	}

	if fieldName, ok := s.config.Build.Secrets.FromStored[secret]; ok {
		if fieldName == "" {
			fieldName = secret
		}

		if s.secrets == nil {
			secretLock.Lock()

			defer secretLock.Unlock()

			kv, err := docker.CreateKVConnection(ctx, s.remoteClient)

			if err != nil {
				return nil, err
			}

			secrets, err := docker.ListProjectSecrets(kv, s.config.Name)

			if err != nil {
				return nil, err
			}

			kv.Close()

			s.secrets = secrets
		}

		if val, ok := s.secrets[fieldName]; ok {
			return []byte(val), nil
		}

		return nil, fmt.Errorf("could not found value for secret %s: using stored value %s", secret, fieldName)
	}

	return nil, fmt.Errorf("could not found source for secret \"%s\". Did you maybe forgot to add the secret to your .tanjun.yml", secret)
}
