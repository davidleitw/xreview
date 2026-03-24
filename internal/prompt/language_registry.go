package prompt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/davidleitw/xreview/internal/prompt/languages"
)

// SupportedLanguages maps language keys (used in --language flag) to display names.
var SupportedLanguages = map[string]string{
	"cpp": "C++",
	"go":  "Go",
}

// SupportedLanguageList returns a comma-separated list of supported language keys.
func SupportedLanguageList() string {
	keys := make([]string, 0, len(SupportedLanguages))
	for k := range SupportedLanguages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// loadLanguageContent reads the embedded .md file for the given language key.
func loadLanguageContent(lang string) (string, error) {
	filename := lang + ".md"
	data, err := languages.FS.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("load language guidelines for %q: %w", lang, err)
	}
	return string(data), nil
}
