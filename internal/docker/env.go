package docker

import (
	"context"
	"fmt"
	"math/rand"
	"os"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/shyim/tanjun/internal/onepassword"

	"github.com/expr-lang/expr"
)

func prepareEnvironmentVariables(ctx context.Context, cfg DeployConfiguration) error {
	context := map[string]interface{}{
		"randomString": randomString,
		"config":       cfg.ProjectConfig,
		"service":      cfg.serviceConfig,
	}

	if err := resolveEnvFromExpression(cfg, context); err != nil {
		return err
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

		cfg.environmentVariables[key] = envValue
	}

	if err := resolveEnvFromFile(cfg); err != nil {
		return err
	}

	if err := resolveInitialSecrets(cfg, context); err != nil {
		return err
	}

	if err := resolveOnePasswordSecrets(ctx, cfg); err != nil {
		return err
	}

	return nil
}

func resolveOnePasswordSecrets(ctx context.Context, cfg DeployConfiguration) error {
	for _, secret := range cfg.ProjectConfig.App.Secrets.OnePassword.Secret {
		onePasswordSecrets, err := onepassword.ResolveSecrets(ctx, secret)

		if err != nil {
			return err
		}

		for key, value := range onePasswordSecrets {
			cfg.environmentVariables[key] = value
		}
	}
	return nil
}

func resolveEnvFromExpression(cfg DeployConfiguration, context map[string]interface{}) error {
	for key, value := range cfg.ProjectConfig.App.Environment {
		if value.Value != "" {
			cfg.environmentVariables[key] = value.Value

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

		cfg.environmentVariables[key] = output.(string)
	}
	return nil
}

func resolveEnvFromFile(cfg DeployConfiguration) error {
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
			cfg.environmentVariables[key] = value
		}
	}

	return nil
}

func resolveInitialSecrets(cfg DeployConfiguration, context map[string]interface{}) error {
	secrets, err := ListProjectSecrets(cfg.storage, cfg.Name)

	if err != nil {
		return err
	}

	changed := false

	for key, value := range cfg.ProjectConfig.App.InitialSecrets {
		if _, ok := secrets[key]; ok {
			cfg.environmentVariables[key] = secrets[key]

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

		cfg.environmentVariables[key] = output.(string)
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
