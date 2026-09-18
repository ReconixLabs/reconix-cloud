package model

import "time"

type Status string

const (
	Queued    Status = "queued"
	Starting  Status = "starting"
	Running   Status = "running"
	Completed Status = "completed"
	Failed    Status = "failed"
	Cancelled Status = "cancelled"
)

type Scan struct {
	ID          string     `json:"scan_id"`
	Target      string     `json:"target"`
	Profile     string     `json:"profile"`
	Status      Status     `json:"status"`
	Stage       string     `json:"stage,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Error       string     `json:"error,omitempty"`
}

type Finding struct {
	FindingID string         `json:"finding_id"`
	Source    string         `json:"source"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Severity  string         `json:"severity"`
	Asset     map[string]any `json:"asset,omitempty"`
	Evidence  map[string]any `json:"evidence,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type NormalizedResult struct {
	SchemaVersion string    `json:"schema_version"`
	ScanID        string    `json:"scan_id"`
	Target        string    `json:"target"`
	Findings      []Finding `json:"findings"`
}
