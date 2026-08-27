package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSONMap is a map[string]any that stores itself as JSON in a single text column.
// It implements driver.Valuer and sql.Scanner so GORM can serialize it transparently.
type JSONMap map[string]any

// Value marshals the map to JSON for the database driver.
// A nil map is stored as a JSON null rather than an empty object so callers can
// distinguish "unset" from "explicitly empty".
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "null", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("JSONMap marshal: %w", err)
	}
	return string(b), nil
}

// Scan unmarshals JSON from the database into the map. It accepts []byte, string,
// and the nil literal, and is a no-op for already-empty data.
func (m *JSONMap) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return errors.New("JSONMap.Scan: unsupported source type")
	}
	if len(raw) == 0 {
		*m = nil
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("JSONMap unmarshal: %w", err)
	}
	*m = out
	return nil
}

// MarshalJSON returns the underlying map JSON, emitting null for nil.
func (m JSONMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]any(m))
}

// UnmarshalJSON copies raw JSON into the map.
func (m *JSONMap) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*m = nil
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

// StringArray is a []string stored as a JSON array in a single text column,
// standing in for PostgreSQL's VARCHAR[] type on SQLite.
type StringArray []string

// Value marshals the slice to JSON.
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("StringArray marshal: %w", err)
	}
	return string(b), nil
}

// Scan unmarshals a JSON array from the database into the slice.
func (a *StringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return errors.New("StringArray.Scan: unsupported source type")
	}
	if len(raw) == 0 {
		*a = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("StringArray unmarshal: %w", err)
	}
	*a = out
	return nil
}

// MarshalJSON returns the JSON array form, defaulting to [] for nil.
func (a StringArray) MarshalJSON() ([]byte, error) {
	if a == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(a))
}

// UnmarshalJSON copies raw JSON into the slice.
func (a *StringArray) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*a = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*a = out
	return nil
}
