package catalog

import (
	"bytes"
	"encoding/json"
)

// defaultLang is the fallback locale: content always carries en-US, other locales
// are optional and fall back to it.
const defaultLang = "en-US"

// localizedText is a per-locale string map that also accepts a bare string for
// back-compat: older curated entries store `"description": "..."`, newer ones
// store `"description": {"en-US": "...", "pt-BR": "..."}`. Both unmarshal here.
type localizedText map[string]string

// UnmarshalJSON accepts either a JSON string (stored under the default locale) or
// a {locale: text} object.
func (l *localizedText) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*l = localizedText{defaultLang: s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*l = m
	return nil
}

// get returns the text for a locale, falling back to the default locale and then
// any non-empty value.
func (l localizedText) get(lang string) string {
	if v, ok := l[lang]; ok && v != "" {
		return v
	}
	if v, ok := l[defaultLang]; ok && v != "" {
		return v
	}
	for _, v := range l {
		if v != "" {
			return v
		}
	}
	return ""
}
