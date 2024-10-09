package docker

import (
	"encoding/json"
	"fmt"
	"github.com/gosimple/slug"
)

func ListProjectSecrets(kv *KvClient, name string) (map[string]string, error) {
	cfg := DeployConfiguration{Name: slug.Make(name)}

	secret := kv.Get(cfg.ContainerPrefix() + "_secrets")

	if secret == "" {
		return make(map[string]string), nil
	}

	var secrets map[string]string

	if err := json.Unmarshal([]byte(secret), &secrets); err != nil {
		return nil, err
	}

	return secrets, nil
}

func SetProjectSecrets(kv *KvClient, name string, secrets map[string]string) error {
	cfg := DeployConfiguration{Name: slug.Make(name)}

	secret, err := json.Marshal(secrets)

	if err != nil {
		return err
	}

	if kv.Set(cfg.ContainerPrefix()+"_secrets", string(secret)) == false {
		return fmt.Errorf("could not set secrets")
	}

	return nil
}
