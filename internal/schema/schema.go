package schema

import (
	_ "embed"
	"fmt"
	"os"
)

//go:embed review.json
var ReviewSchemaBytes []byte

// WriteTempSchema writes the embedded JSON schema to a temp file.
// Returns the file path and a cleanup function. Caller must call cleanup after use.
func WriteTempSchema() (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "xreview-schema-*.json")
	if err != nil {
		return "", nil, fmt.Errorf("create temp schema file: %w", err)
	}

	if _, err := f.Write(ReviewSchemaBytes); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("write temp schema file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("close temp schema file: %w", err)
	}

	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
