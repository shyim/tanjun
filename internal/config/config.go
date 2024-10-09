package config

import (
	"fmt"
	"os"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

type ProjectConfig struct {
	Name      string `yaml:"name" jsonschema:"required"`
	ImageName string `yaml:"imageName" jsonschema:"required"`
	Server    struct {
		Address  string `yaml:"address" jsonschema:"required"`
		Username string `yaml:"username"`
		Port     int    `yaml:"port"`
	} `yaml:"server" jsonschema:"required"`
	Proxy struct {
		Host           string `yaml:"host"`
		Port           int    `yaml:"port"`
		HealthCheckUrl string `yaml:"healthCheckUrl"`
	} `yaml:"proxy"`
	LocalBuild bool                      `yaml:"localBuild"`
	App        ProjectApp                `yaml:"app"`
	Services   map[string]ProjectService `yaml:"services"`
}

type ProjectWorker struct {
	Command  string `yaml:"command"`
	Replicas int    `yaml:"replicas"`
}

type ProjectCronjob struct {
	Schedule string `yaml:"schedule"`
	Command  string `yaml:"command"`
}

type ProjectApp struct {
	Dockerfile     string                           `yaml:"dockerFile"`
	Environment    map[string]ProjectEnvironment    `yaml:"env"`
	InitialSecrets map[string]ProjectInitialSecrets `yaml:"initialSecrets"`
	Mounts         []ProjectMount                   `yaml:"mounts"`
	Workers        map[string]ProjectWorker         `yaml:"workers"`
	Cronjobs       []ProjectCronjob                 `yaml:"cronjobs"`
	Hooks          struct {
		Setup   string `yaml:"setup"`
		Changed string `yaml:"changed"`
	} `yaml:"hooks"`
}

type ProjectInitialSecrets struct {
	Expression string `yaml:"expr" jsonschema:"required"`
}

type ProjectEnvironment struct {
	Value      string `yaml:"value" jsonschema:"oneof_required=value"`
	Expression string `yaml:"expr" jsonschema:"oneof_required=expr"`
}

type ProjectMount struct {
	Name string `yaml:"name" jsonschema:"required"`
	Path string `yaml:"path" jsonschema:"required"`
}

type ProjectService struct {
	Type     string            `yaml:"type" jsonschema:"enum=mysql:8.0,enum=mysql:8.4,enum=valkey:7.2"`
	Settings map[string]string `yaml:"settings"`
}

func CreateConfig(file string) (*ProjectConfig, error) {
	var cfg ProjectConfig

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file %s does not exist", file)
	}

	data, err := os.ReadFile(file)

	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	fillDefaults(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	if err := validateCronjobs(cfg.App.Cronjobs); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateConfig(projectConfig *ProjectConfig) error {
	if projectConfig.Name == "" {
		return fmt.Errorf("missing project name")
	}

	if projectConfig.ImageName == "" {
		return fmt.Errorf("missing image name")
	}

	if projectConfig.Server.Address == "" {
		return fmt.Errorf("missing server address")
	}

	if projectConfig.Proxy.Host == "" {
		return fmt.Errorf("missing proxy host")
	}

	return nil
}

func validateCronjobs(cronjobs []ProjectCronjob) error {
	for i, cronjob := range cronjobs {
		if _, err := cron.ParseStandard(cronjob.Schedule); err != nil {
			return fmt.Errorf("cronjob[%d]: %w", i, err)
		}
	}

	return nil
}

func fillDefaults(p *ProjectConfig) {
	if p.App.Environment == nil {
		p.App.Environment = make(map[string]ProjectEnvironment)
	}

	if p.App.Dockerfile == "" {
		p.App.Dockerfile = "Dockerfile"
	}

	if p.Server.Port == 0 {
		p.Server.Port = 22
	}

	if p.Server.Username == "" {
		p.Server.Username = "root"
	}

	if p.Proxy.HealthCheckUrl == "" {
		p.Proxy.HealthCheckUrl = "/"
	}
}
