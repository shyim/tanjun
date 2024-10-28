package buildpack

import "github.com/shyim/go-version"

type PackageJSON struct {
	Main            string            `json:"main"`
	PackageManager  string            `json:"packageManager"`
	Engines         map[string]string `json:"engines"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func (j PackageJSON) HasDependencies() bool {
	return len(j.Dependencies) > 0 || len(j.DevDependencies) > 0
}

func detectNodeVersion(packageJson PackageJSON) string {
	nodeConstraint, ok := packageJson.Engines["node"]

	if !ok {
		return "22"
	}

	constraint, err := version.NewConstraint(nodeConstraint)

	if err != nil {
		return "22"
	}

	if constraint.Check(version.Must(version.NewVersion("23"))) {
		return "23"
	}

	if constraint.Check(version.Must(version.NewVersion("22"))) {
		return "22"
	}

	if constraint.Check(version.Must(version.NewVersion("20"))) {
		return "20"
	}

	return "18"
}
