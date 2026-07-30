package store

import (
	"time"

	"gorm.io/gorm"
)

// ChainRecord persists a scenario chain definition as a JSON blob.
type ChainRecord struct {
	ID          string `gorm:"primaryKey"`
	Name        string
	Description string
	Data        string    `gorm:"type:text"` // JSON of chain.Chain
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// ExecutionRecord tracks a single execution of a chain against a target session.
type ExecutionRecord struct {
	ID         string  `gorm:"primaryKey"`
	ChainID    string  `gorm:"index"`
	SessionID  string
	Status     string // pending | running | done | failed | cancelled
	Error      string
	StartedAt  time.Time
	FinishedAt *time.Time
}

// StepLog records the result of a single step within an execution.
type StepLog struct {
	gorm.Model
	ExecutionID string `gorm:"index"`
	StepID      string
	Status      string // running | done | failed | skipped
	Stdout      string `gorm:"type:text"`
	Stderr      string `gorm:"type:text"`
	ExitCode    int
	Error       string
	DurationMs  int64
}
