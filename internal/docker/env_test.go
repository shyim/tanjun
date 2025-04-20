package docker

import (
	"os"
	"testing"

	"github.com/shyim/tanjun/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestResolveEnvFromExpression(t *testing.T) {
	// Setup a simple config
	projectConfig := &config.ProjectConfig{
		Name: "test-project",
	}

	deployConfig := DeployConfiguration{
		Name:          "test-project",
		ProjectConfig: projectConfig,
		serviceConfig: map[string]interface{}{
			"param1": "value1",
		},
		storedSecrets: make(map[string]string),
	}

	environmentVariables := map[string]config.ProjectEnvironment{
		"SIMPLE_VALUE": {
			Value: "simple-value",
		},
		"EXPRESSION_VALUE": {
			Expression: `randomString(10)`,
		},
		"CONFIG_VALUE": {
			Expression: `config.Name`,
		},
		"SERVICE_VALUE": {
			Expression: `service.param1`,
		},
	}

	// Execute
	result := make(map[string]string)
	err := resolveEnvFromExpression(deployConfig, result, map[string]interface{}{
		"randomString": randomString,
		"config":       deployConfig.ProjectConfig,
		"service":      deployConfig.serviceConfig,
	}, environmentVariables)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "simple-value", result["SIMPLE_VALUE"])
	assert.Len(t, result["EXPRESSION_VALUE"], 10) // Random string of length 10
	assert.Equal(t, "test-project", result["CONFIG_VALUE"])
	assert.Equal(t, "value1", result["SERVICE_VALUE"])
}

func TestResolveFromStoredSecrets(t *testing.T) {
	// Setup with stored secrets
	deployConfig := DeployConfiguration{
		Name: "test-project",
		storedSecrets: map[string]string{
			"stored_secret1": "stored-value1",
			"stored_secret2": "stored-value2",
		},
	}

	genericSecrets := config.ProjectGenericSecrets{
		FromStored: map[string]string{
			"ENV_SECRET1": "stored_secret1",
			"ENV_SECRET2": "",             // Should use the key name as stored key (which doesn't exist)
			"ENV_SECRET3": "non_existent", // Tests missing secret
		},
	}

	// Execute
	result := make(map[string]string)
	err := resolveFromStoredSecrets(result, deployConfig, genericSecrets)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "stored-value1", result["ENV_SECRET1"])
	assert.NotContains(t, result, "ENV_SECRET2") // Key doesn't exist in stored secrets
	assert.NotContains(t, result, "ENV_SECRET3") // Key doesn't exist in stored secrets
}

func TestResolveEnvFromFile(t *testing.T) {
	// Create temp env file
	tmpFile, err := os.CreateTemp("", "env-test")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write test env contents
	_, err = tmpFile.WriteString("FILE_VAR1=file-value1\nFILE_VAR2=file-value2\n")
	assert.NoError(t, err)
	tmpFile.Close()

	genericSecrets := config.ProjectGenericSecrets{
		FromEnvFile: []string{
			tmpFile.Name(),
			"non-existent-file", // Testing missing file handling
		},
	}

	// Execute
	result := make(map[string]string)
	err = resolveEnvFromFile(result, genericSecrets)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "file-value1", result["FILE_VAR1"])
	assert.Equal(t, "file-value2", result["FILE_VAR2"])
}
