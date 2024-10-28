package buildpack

func addEnvFromSettings(image *GeneratedImageResult, cfg *Config) {
	envMap := cfg.Settings["env"].(ConfigSettings)

	if len(envMap) == 0 {
		return
	}

	envLine := ""

	for key, val := range envMap {
		envLine += " " + key + "=" + val.(string)
	}

	image.AddLine("ENV %s", envLine)
}

func addPackagesFromSettings(image *GeneratedImageResult, cfg *Config, additionalPackages string) {
	for _, pkg := range cfg.Settings["packages"].([]interface{}) {
		additionalPackages += " " + pkg.(string)
	}

	if additionalPackages != "" {
		image.AddLine("RUN apk add --no-cache %s", additionalPackages)
	}
}
