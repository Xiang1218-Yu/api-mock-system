// Package models defines the persistence entities for the entire domain.
// Every type here is a plain struct with no business logic: it represents a row
// in the database and nothing else. Logic lives in services and repositories.
//
// Tables map directly to the schema in system.md §5. Because we use SQLite
// instead of PostgreSQL, the `tags VARCHAR[]` array column is stored as a
// JSON-encoded string (DATETIME('json') handled by GORM serialization hooks).
package models

import (
	"time"
)

// Base is embedded by every entity that owns its own primary key and timestamps.
type Base struct {
	ID        string    `gorm:"type:text;primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents an account that can authenticate and own/be a member of projects.
type User struct {
	Base
	Email        string `gorm:"type:text;uniqueIndex" json:"email"`
	PasswordHash string `gorm:"type:text" json:"-"`
	Name         string `gorm:"type:text" json:"name"`
}

// Project is the top-level container for APIs and aggregates.
type Project struct {
	Base
	Name        string `gorm:"type:text" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	BasePath    string `gorm:"type:text" json:"base_path"`
	OwnerID     string `gorm:"type:text" json:"owner_id"`
	Visibility  string `gorm:"type:text" json:"visibility"` // public|private
}

// ProjectMember links a user to a project with an assigned role.
type ProjectMember struct {
	Base
	ProjectID string `gorm:"type:text;uniqueIndex" json:"project_id"`
	UserID    string `gorm:"type:text;index" json:"user_id"`
	Role      string `gorm:"type:text" json:"role"` // admin|editor|viewer
}

// API is an endpoint definition owned by a project.
type API struct {
	Base
	ProjectID       string      `gorm:"type:text;index" json:"project_id"`
	Name            string      `gorm:"type:text" json:"name"`
	Description     string      `gorm:"type:text" json:"description"`
	Method          string      `gorm:"type:text" json:"method"`
	Path            string      `gorm:"type:text" json:"path"`
	Status          string      `gorm:"type:text" json:"status"` // designing|published|deprecated
	RequestSchema   JSONMap     `gorm:"type:text;serializer:json" json:"request_schema"`
	ResponseSchema  JSONMap     `gorm:"type:text;serializer:json" json:"response_schema"`
	ResponseExample JSONMap     `gorm:"type:text;serializer:json" json:"response_example"`
	MockDelay       int         `json:"mock_delay"`
	MockStatusCode  int         `json:"mock_status_code"`
	GroupID         *string     `gorm:"type:text" json:"group_id,omitempty"`
	Tags            StringArray `gorm:"type:text;serializer:json" json:"tags"`
	Version         int         `json:"version"`
}

// APIVersion is an immutable snapshot of an API definition at a point in time.
type APIVersion struct {
	Base
	APIID         string  `gorm:"type:text;index" json:"api_id"`
	Version       int     `json:"version"`
	Snapshot      JSONMap `gorm:"type:text;serializer:json" json:"snapshot"`
	ChangeComment string  `gorm:"type:text" json:"change_comment"`
	CreatedBy     string  `gorm:"type:text" json:"created_by"`
}

// Aggregate is a composite endpoint that fans out to multiple downstream APIs.
type Aggregate struct {
	Base
	ProjectID      string  `gorm:"type:text;index" json:"project_id"`
	Name           string  `gorm:"type:text" json:"name"`
	Description    string  `gorm:"type:text" json:"description"`
	Path           string  `gorm:"type:text" json:"path"`
	Mode           string  `gorm:"type:text" json:"mode"` // serial|parallel|conditional
	Timeout        int     `json:"timeout"`               // milliseconds
	DownstreamAPIs JSONMap `gorm:"type:text;serializer:json" json:"downstream_apis"`
	FieldMappings  JSONMap `gorm:"type:text;serializer:json" json:"field_mappings"`
}

// MockData is a fixed override that replaces generated mock output for a key.
type MockData struct {
	Base
	APIID   string  `gorm:"type:text;index" json:"api_id"`
	Key     string  `gorm:"type:text" json:"key"`
	Value   JSONMap `gorm:"type:text;serializer:json" json:"value"`
	Enabled bool    `json:"enabled"`
}

// DebugLog records a single debug invocation for later replay and inspection.
type DebugLog struct {
	Base
	UserID      string  `gorm:"type:text;index" json:"user_id"`
	APIID       *string `gorm:"type:text" json:"api_id,omitempty"`
	AggregateID *string `gorm:"type:text" json:"aggregate_id,omitempty"`
	Request     JSONMap `gorm:"type:text;serializer:json" json:"request"`
	Response    JSONMap `gorm:"type:text;serializer:json" json:"response"`
	StatusCode  int     `json:"status_code"`
	Duration    int     `json:"duration"` // milliseconds
}

// CallLog records one runtime call to a mock or aggregate endpoint, feeding the
// dashboard's call-count, trend, and latency-distribution metrics (spec §2.7).
// Writes are best-effort and asynchronous, so a write hiccup never fails a
// mock/aggregate call.
type CallLog struct {
	Base
	ProjectID   string  `gorm:"type:text;index" json:"project_id"`
	Kind        string  `gorm:"type:text;index" json:"kind"` // "mock" | "aggregate"
	APIID       *string `gorm:"type:text" json:"api_id,omitempty"`
	AggregateID *string `gorm:"type:text" json:"aggregate_id,omitempty"`
	Method      string  `gorm:"type:text" json:"method"`
	Path        string  `gorm:"type:text" json:"path"`
	StatusCode  int     `json:"status_code"`
	Duration    int     `json:"duration"` // milliseconds
}
