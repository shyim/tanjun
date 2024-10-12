package docker

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"math/rand"
	"os"

	"github.com/expr-lang/expr"
)

func prepareEnvironmentVariables(cfg DeployConfiguration) error {
	context := map[string]interface{}{
		"randomString": randomString,
	}

	secrets, err := ListProjectSecrets(cfg.storage, cfg.Name)

	if err != nil {
		return err
	}

	for key, value := range cfg.ProjectConfig.App.Environment {
		if value.Value != "" {
			cfg.EnvironmentVariables[key] = value.Value

			continue
		}

		program, err := expr.Compile(value.Expression, expr.Env(context))
		if err != nil {
			return err
		}

		output, err := expr.Run(program, context)
		if err != nil {
			return err
		}

		cfg.EnvironmentVariables[key] = output.(string)
	}

	for key, value := range cfg.ProjectConfig.App.Secrets.FromEnv {
		if value == "" {
			value = key
		}

		envValue := os.Getenv(value)

		if envValue == "" {
			log.Warnf("Environment variable %s is not set, skipping setting a value", value)

			continue
		}

		cfg.EnvironmentVariables[key] = envValue
	}

	for _, fileName := range cfg.ProjectConfig.App.Secrets.FromEnvFile {
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			log.Warnf("Environment file %s does not exist, skipping setting a value", fileName)

			continue
		}

		envMap, err := godotenv.Read(fileName)

		if err != nil {
			return fmt.Errorf("error reading environment file %s: %w", fileName, err)
		}

		for key, value := range envMap {
			cfg.EnvironmentVariables[key] = value
		}
	}

	changed := false

	for key, value := range cfg.ProjectConfig.App.InitialSecrets {
		if _, ok := secrets[key]; ok {
			cfg.EnvironmentVariables[key] = secrets[key]

			continue
		}

		program, err := expr.Compile(value.Expression, expr.Env(context))
		if err != nil {
			return err
		}

		output, err := expr.Run(program, context)
		if err != nil {
			return err
		}

		cfg.EnvironmentVariables[key] = output.(string)
		secrets[key] = output.(string)

		changed = true
	}

	if changed {
		if err := SetProjectSecrets(cfg.storage, cfg.Name, secrets); err != nil {
			return err
		}
	}

	return nil
}

func randomString(n int) string {
	const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return string(b)
}
