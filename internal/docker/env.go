package docker

import (
	"context"
	"fmt"
	"math/rand"
	"os"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/onepassword"

	"github.com/expr-lang/expr"
)

func getEnvironmentVariables(ctx context.Context, cfg DeployConfiguration, environmentVariables map[string]config.ProjectEnvironment, genericSecret config.ProjectGenericSecrets, initialSecrets map[string]config.ProjectInitialSecrets) (map[string]string, error) {
	context := map[string]interface{}{
		"randomString": randomString,
		"config":       cfg.ProjectConfig,
		"service":      cfg.serviceConfig,
	}

	returnSecrets := make(map[string]string)

	if err := resolveEnvFromExpression(cfg, returnSecrets, context, environmentVariables); err != nil {
		return nil, err
	}

	for key, value := range genericSecret.FromEnv {
		if value == "" {
			value = key
		}

		envValue := os.Getenv(value)

		if envValue == "" {
			log.Warnf("Environment variable %s is not set, skipping setting a value", value)

			continue
		}

		returnSecrets[key] = envValue
	}

	if err := resolveFromStoredSecrets(returnSecrets, cfg, genericSecret); err != nil {
		return nil, err
	}

	if err := resolveEnvFromFile(returnSecrets, genericSecret); err != nil {
		return nil, err
	}

	if err := resolveInitialSecrets(returnSecrets, cfg, context, initialSecrets); err != nil {
		return nil, err
	}

	if err := resolveOnePasswordSecrets(ctx, returnSecrets, genericSecret); err != nil {
		return nil, err
	}

	return returnSecrets, nil
}

func resolveFromStoredSecrets(returnSecrets map[string]string, cfg DeployConfiguration, genericSecrets config.ProjectGenericSecrets) error {
	for key, value := range genericSecrets.FromStored {
		if value == "" {
			value = key
		}

		if _, ok := cfg.storedSecrets[value]; ok {
			returnSecrets[key] = cfg.storedSecrets[value]

			continue
		}

		log.Warnf("Secret %s is not set, skipping setting a value", value)
	}

	return nil
}

func resolveOnePasswordSecrets(ctx context.Context, returnSecrets map[string]string, genericSecrets config.ProjectGenericSecrets) error {
	for _, secret := range genericSecrets.OnePassword.Secret {
		onePasswordSecrets, err := onepassword.ResolveSecrets(ctx, secret)

		if err != nil {
			return err
		}

		for key, value := range onePasswordSecrets {
			returnSecrets[key] = value
		}
	}
	return nil
}

func resolveEnvFromExpression(cfg DeployConfiguration, returnSecrets map[string]string, context map[string]interface{}, environment map[string]config.ProjectEnvironment) error {
	for key, value := range environment {
		if value.Value != "" {
			returnSecrets[key] = value.Value

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

		returnSecrets[key] = output.(string)
	}
	return nil
}

func resolveEnvFromFile(returnSecrets map[string]string, genericSecrets config.ProjectGenericSecrets) error {
	for _, fileName := range genericSecrets.FromEnvFile {
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			log.Warnf("Environment file %s does not exist, skipping setting a value", fileName)

			continue
		}

		envMap, err := godotenv.Read(fileName)

		if err != nil {
			return fmt.Errorf("error reading environment file %s: %w", fileName, err)
		}

		for key, value := range envMap {
			returnSecrets[key] = value
		}
	}

	return nil
}

func resolveInitialSecrets(returnSecrets map[string]string, cfg DeployConfiguration, context map[string]interface{}, initialSecrets map[string]config.ProjectInitialSecrets) error {
	changed := false

	for key, value := range initialSecrets {
		if _, ok := returnSecrets[key]; ok {
			returnSecrets[key] = cfg.storedSecrets[key]

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

		returnSecrets[key] = output.(string)
		cfg.storedSecrets[key] = output.(string)

		changed = true
	}

	if changed {
		if err := SetProjectSecrets(cfg.storage, cfg.Name, cfg.storedSecrets); err != nil {
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
