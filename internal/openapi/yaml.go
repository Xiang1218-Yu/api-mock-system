package openapi

import "github.com/goccy/go-yaml"

// ToYAML renders a document to YAML bytes. Uses goccy/go-yaml, already an
// indirect dependency of gin, so no new dependency tree is introduced.
func ToYAML(doc *Document) ([]byte, error) {
	return yaml.Marshal(doc)
}
