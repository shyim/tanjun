package config

func AddBuildPackConfig(pc *ProjectConfig, language string) error {
	switch language {
	case "shopware":
		addShopwareConfig(pc)
	}

	return nil
}

func addShopwareConfig(pc *ProjectConfig) {
	pc.App.Environment = map[string]ProjectEnvironment{
		"APP_URL": {
			Expression: `"https://" + config.Proxy.Host`,
		},
		"APP_ENV": {
			Value: "prod",
		},
		"SHOPWARE_HTTP_CACHE_ENABLED": {
			Value: "1",
		},
		"DATABASE_PERSISTENT_CONNECTION": {
			Value: "1",
		},
		"SQL_SET_DEFAULT_SESSION_VARIABLES": {
			Value: "0",
		},
		"ENABLE_SERVICES": {
			Value: "0",
		},
		"DATABASE_URL": {
			Expression: `service.database.url`,
		},
	}

	pc.App.InitialSecrets = map[string]ProjectInitialSecrets{
		"APP_SECRET": {
			Expression: `randomString(32)`,
		},
	}

	pc.App.Mounts = map[string]ProjectMount{
		"jwt": {

			Path: "config/jwt",
		},
		"files": {
			Path: "files",
		},
		"bundles": {
			Path: "public/bundles",
		},
		"theme": {
			Path: "public/theme",
		},
		"media": {
			Path: "public/media",
		},
		"thumbnail": {
			Path: "public/thumbnail",
		},
		"sitemap": {
			Path: "public/sitemap",
		},
	}

	pc.App.Workers = map[string]ProjectWorker{
		"worker": {
			Command: "php bin/console messenger:consume async --time-limit=3600",
		},
	}

	pc.App.Cronjobs = []ProjectCronjob{
		{
			Name:     "scheduled-task",
			Schedule: "@every 5m",
			Command:  "php bin/console scheduled-task:run --no-wait",
		},
	}

	pc.App.Hooks.Deploy = "./vendor/bin/shopware-deployment-helper run --skip-theme-compile"
	pc.App.Hooks.PostDeploy = "./bin/console theme:compile --active-only"

	pc.Services = map[string]ProjectService{
		"database": {
			Type: "mysql:8.0",
		},
	}

	pc.Proxy.HealthCheck.Path = "/admin"
}
