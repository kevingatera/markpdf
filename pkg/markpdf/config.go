// config.go loads markpdf.yaml files into normalized Options used by the CLI
// and public library callers.
package markpdf

import (
	"os"

	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) (Options, error) {
	opts := DefaultOptions()
	if path == "" {
		return opts, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return opts, err
	}
	if err := yaml.Unmarshal(data, &opts); err != nil {
		return opts, err
	}
	return opts.normalized(), nil
}
