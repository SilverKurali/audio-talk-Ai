package asr

// FieldType represents the UI input type for a config field.
type FieldType string

const (
	FieldText   FieldType = "text"
	FieldSecret FieldType = "secret"
	FieldSelect FieldType = "select"
)

// FieldDef describes one configurable field for a provider.
type FieldDef struct {
	Key      string    `json:"key"`                // config key, e.g. "app_id"
	Label    string    `json:"label"`              // UI display label
	Help     string    `json:"help,omitempty"`     // tooltip/hint
	Type     FieldType `json:"type"`               // text, secret, select
	Default  string    `json:"default,omitempty"`  // default value
	Options  []string  `json:"options,omitempty"`  // option values for select type
	Labels   []string  `json:"labels,omitempty"`   // display labels for select options
	Secret   bool      `json:"secret,omitempty"`   // needs encryption at rest
}

// ProviderMeta holds all UI-facing metadata for a provider.
type ProviderMeta struct {
	DisplayName string     `json:"display_name"` // e.g. "讯飞星火"
	Fields      []FieldDef `json:"fields"`       // config field definitions
}
