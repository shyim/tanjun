package config

import (
	"fmt"
	"os"

	"github.com/invopop/jsonschema"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

type ProjectConfig struct {
	Name         string `yaml:"name" jsonschema:"required"`
	Image        string `yaml:"image" jsonschema:"required"`
	KeepVersions int    `yaml:"keep_versions"`
	Build        struct {
		Labels               map[string]string `yaml:"labels,omitempty"`
		BuildArgs            map[string]string `yaml:"args,omitempty"`
		PassThroughSSHSocket bool              `yaml:"passthroughs_ssh_socket,omitempty"`
		Secrets              struct {
			FromEnv     ProjectFromEnv `yaml:"from_env,omitempty"`
			FromStored  ProjectFromEnv `yaml:"from_stored,omitempty"`
			OnePassword struct {
				Secret []ProjectOnePassword `yaml:"items,omitempty"`
			} `yaml:"onepassword,omitempty"`
		} `yaml:"secrets,omitempty"`
	} `yaml:"build,omitempty"`
	Server   ProjectServer             `yaml:"server" jsonschema:"required"`
	Proxy    ProjectProxy              `yaml:"proxy"`
	App      ProjectApp                `yaml:"app,omitempty"`
	Services map[string]ProjectService `yaml:"services,omitempty"`
}

type ProjectServer struct {
	Address  string `yaml:"address" jsonschema:"required"`
	Username string `yaml:"username,omitempty"`
	Port     int    `yaml:"port,omitempty"`
}

type ProjectProxy struct {
	Host        string `yaml:"host,omitempty" jsonschema:"required"`
	AppPort     int    `yaml:"app_port,omitempty"`
	HealthCheck struct {
		Interval int    `yaml:"interval,omitempty"`
		Timeout  int    `yaml:"timeout,omitempty"`
		Path     string `yaml:"path,omitempty"`
	} `yaml:"healthcheck,omitempty"`
	ResponseTimeout int  `yaml:"response_timeout,omitempty"`
	SSL             bool `yaml:"ssl,omitempty"`
	Buffering       struct {
		Requests        bool `yaml:"requests,omitempty"`
		Responses       bool `yaml:"responses,omitempty"`
		MaxRequestBody  int  `yaml:"max_request_body,omitempty"`
		MaxResponseBody int  `yaml:"max_response_body,omitempty"`
		Memory          int  `yaml:"memory,omitempty"`
	} `yaml:"buffering,omitempty"`
}

func (p ProjectProxy) GetURL() string {
	if p.SSL {
		return "https://" + p.Host
	}

	return "http://" + p.Host
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
	Dockerfile     string                           `yaml:"dockerfile"`
	Environment    map[string]ProjectEnvironment    `yaml:"env,omitempty"`
	InitialSecrets map[string]ProjectInitialSecrets `yaml:"initial_secrets,omitempty"`
	Secrets        struct {
		FromEnv     ProjectFromEnv `yaml:"from_env,omitempty"`
		FromEnvFile []string       `yaml:"from_env_file,omitempty"`
		OnePassword struct {
			Secret []ProjectOnePassword `yaml:"items,omitempty"`
		} `yaml:"onepassword,omitempty"`
	} `yaml:"secrets,omitempty"`
	Mounts   []ProjectMount           `yaml:"mounts,omitempty"`
	Workers  map[string]ProjectWorker `yaml:"workers,omitempty"`
	Cronjobs []ProjectCronjob         `yaml:"cronjobs,omitempty"`
	Hooks    struct {
		Deploy     string `yaml:"deploy,omitempty"`
		PostDeploy string `yaml:"post_deploy,omitempty"`
	} `yaml:"hooks,omitempty"`
}

type ProjectOnePassword struct {
	Name        string            `yaml:"name" jsonschema:"required"`
	Vault       string            `yaml:"vault" jsonschema:"required"`
	OmitFields  []string          `yaml:"omit_fields,omitempty"`
	RemapFields map[string]string `yaml:"remap_fields,omitempty"`
}

type ProjectFromEnv map[string]string

func (e ProjectFromEnv) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		AdditionalProperties: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{
					Type: "string",
				},
				{
					Type: "null",
				},
			},
		},
	}
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
	Type     string            `yaml:"type"`
	Settings map[string]string `yaml:"settings"`
}

func (e ProjectService) JSONSchema() *jsonschema.Schema {
	return serviceSchema
}

var serviceSchema *jsonschema.Schema

func SetServiceSchema(schema *jsonschema.Schema) {
	serviceSchema = schema
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

	cfg.FillDefaults()

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

	if projectConfig.Image == "" {
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

func (p *ProjectConfig) FillDefaults() {
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

	if p.Proxy.HealthCheck.Path == "" {
		p.Proxy.HealthCheck.Path = "/"
	}

	if p.Proxy.HealthCheck.Timeout == 0 {
		p.Proxy.HealthCheck.Timeout = 5
	}

	if p.Proxy.HealthCheck.Interval == 0 {
		p.Proxy.HealthCheck.Interval = 1
	}

	if p.Proxy.ResponseTimeout == 0 {
		p.Proxy.ResponseTimeout = 30
	}

	if p.KeepVersions == 0 {
		p.KeepVersions = 5
	}
}
