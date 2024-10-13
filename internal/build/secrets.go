package build

import (
	"context"
	"fmt"
	"github.com/shyim/tanjun/internal/config"
	"os"
)

type secretStore struct {
	config *config.ProjectConfig
}

func (s secretStore) GetSecret(ctx context.Context, secret string) ([]byte, error) {
	if useEnv, ok := s.config.Build.Secrets.FromEnv[secret]; ok {
		if useEnv == "" {
			useEnv = secret
		}

		if val, ok := os.LookupEnv(useEnv); ok {
			return []byte(val), nil
		}

		return nil, fmt.Errorf("could not found value for secret %s: using environment value %s", secret, useEnv)
	}

	return nil, fmt.Errorf("could not found source for secret \"%s\". Did you maybe forgot to add the secret to your .tanjun.yml", secret)
}
