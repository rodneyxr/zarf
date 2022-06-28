package validate

import (
	"fmt"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Run performs config validations and runs message.Fatal() on errors
func Run() {
	components := config.GetComponents()

	if err := validatePackageName(config.GetMetaData().Name); err != nil {
		message.Fatalf(err, "Invalid package name")
	}

	uniqueNames := make(map[string]bool)

	for _, component := range components {
		// ensure component name is unique
		if _, ok := uniqueNames[component.Name]; ok {
			message.Fatalf(nil, "Component names must be unique")
		}
		uniqueNames[component.Name] = true

		validateComponent(component)
	}

}

func validateComponent(component types.ZarfComponent) {
	if component.Required {
		if component.Default {
			message.Fatalf(nil, "Component %s cannot be required and default", component.Name)
		}
		if component.Group != "" {
			message.Fatalf(nil, "Component %s cannot be required and part of a choice group", component.Name)
		}
	}

	for _, chart := range component.Charts {
		if err := validateChart(chart); err != nil {
			message.Fatalf(err, "Invalid chart definition in the %s component: %s", component.Name)
		}
	}
	for _, manifest := range component.Manifests {
		if err := validateManifest(manifest); err != nil {
			message.Fatalf(err, "Invalid manifest definition in the %s component: %s", component.Name)
		}
	}
}

func validatePackageName(subject string) error {
	// https://regex101.com/r/vpi8a8/1
	isValid := regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString
	if isValid(subject) {
		return nil
	}
	return fmt.Errorf("package name '%s' must be all lowercase and contain no special characters except -", subject)
}

func validateChart(chart types.ZarfChart) error {
	intro := fmt.Sprintf("chart %s", chart.Name)

	// Don't allow empty names
	if chart.Name == "" {
		return fmt.Errorf("%s must include a name", intro)
	}

	// Helm max release name
	if len(chart.Name) > config.ZarfMaxChartNameLength {
		return fmt.Errorf("%s exceed the maximum length of %d characters",
			intro,
			config.ZarfMaxChartNameLength)
	}

	// Must have a namespace
	if chart.Namespace == "" {
		return fmt.Errorf("%s must include a namespace", intro)
	}

	// Must have a url
	if chart.Url == "" {
		return fmt.Errorf("%s must include a url", intro)
	}

	// Must have a version
	if chart.Version == "" {
		return fmt.Errorf("%s must include a chart version", intro)
	}

	return nil
}

func validateManifest(manifest types.ZarfManifest) error {
	intro := fmt.Sprintf("chart %s", manifest.Name)

	// Don't allow empty names
	if manifest.Name == "" {
		return fmt.Errorf("%s must include a name", intro)
	}

	// Helm max release name
	if len(manifest.Name) > config.ZarfMaxChartNameLength {
		return fmt.Errorf("%s exceed the maximum length of %d characters",
			intro,
			config.ZarfMaxChartNameLength)
	}

	// Require files in manifest
	if len(manifest.Files) < 1 && len(manifest.Kustomizations) < 1 {
		return fmt.Errorf("%s must have at least one file or kustomization", intro)
	}

	return nil
}

func ValidateImportPackage(composedComponent *types.ZarfComponent) error {
	intro := fmt.Sprintf("imported package %s", composedComponent.Name)
	path := composedComponent.Import.Path

	// ensure path exists
	if path == "" {
		return fmt.Errorf("%s must include a path", intro)
	}

	// ensure there is a zarf.yaml in provided path
	if utils.InvalidPath(path) {
		return fmt.Errorf("invalid file path \"%s\" provided directory must contain a valid zarf.yaml file", composedComponent.Import.Path)
	}

	return nil
}
