package config

import (
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectProxy_GetURL(t *testing.T) {
	proxy := ProjectProxy{
		Host: "localhost",
	}

	assert.Equal(t, "http://localhost", proxy.GetURL())

	proxy.SSL = true

	assert.Equal(t, "https://localhost", proxy.GetURL())
}

func TestProxyGetService(t *testing.T) {
	s := ProjectService{}

	assert.Nil(t, s.JSONSchema())

	SetServiceSchema(&jsonschema.Schema{})

	assert.NotNil(t, s.JSONSchema())
}

func TestConfigLoadWithoutExistence(t *testing.T) {
	_, err := CreateConfig("nonexistent.yml")

	assert.Error(t, err)

	test := ProjectFromEnv{}
	assert.NotNil(t, test.JSONSchema())
}

func TestConfigLoadWithInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "invalid.yml"), []byte("invalid"), 0644))

	_, err := CreateConfig(filepath.Join(tmpDir, "invalid.yml"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal")
}

func TestConfigLoadWithValidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "valid.yml"), []byte("name: blaa\nimage: blaa\nserver:\n  address: localhost\nproxy:\n  host: foo.com"), 0644))

	cfg, err := CreateConfig(filepath.Join(tmpDir, "valid.yml"))

	assert.NoError(t, err)
	assert.Equal(t, "blaa", cfg.Name)
}

func TestConfigIncludes(t *testing.T) {
	tmpDir := t.TempDir()

	currentDir, err := os.Getwd()

	assert.NoError(t, err)

	assert.NoError(t, os.Chdir(tmpDir))

	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "base.yml"), []byte("name: blaa\nimage: blaa\nserver:\n  address: localhost\nproxy:\n  host: foo.com"), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "valid.yml"), []byte("include:\n  - base.yml"), 0644))

	cfg, err := CreateConfig(filepath.Join(tmpDir, "valid.yml"))

	assert.NoError(t, err)
	assert.Equal(t, "blaa", cfg.Name)

	assert.NoError(t, os.Chdir(currentDir))
}

func TestConfigIncludeFileMissing(t *testing.T) {
	tmpDir := t.TempDir()

	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "invalid.yml"), []byte("include:\n  - missing.yml"), 0644))

	_, err := CreateConfig(filepath.Join(tmpDir, "invalid.yml"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read include file missing.yml")
}

func TestConfigIncludeFileInvalidYaml(t *testing.T) {
	tmpDir := t.TempDir()

	currentDir, err := os.Getwd()

	assert.NoError(t, err)

	assert.NoError(t, os.Chdir(tmpDir))

	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "base.yml"), []byte("!!!!!!!"), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "valid.yml"), []byte("include:\n  - base.yml"), 0644))

	cfg, err := CreateConfig(filepath.Join(tmpDir, "valid.yml"))

	assert.Error(t, err)
	assert.Nil(t, cfg)

	assert.NoError(t, os.Chdir(currentDir))
}
