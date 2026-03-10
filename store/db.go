package store

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Store wraps a GORM database providing typed access to scenario persistence.
type Store struct {
	db *gorm.DB
}

// Open opens (or creates) a SQLite database at the given path and auto-migrates the schema.
func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", path, err)
	}
	// Enable WAL mode for better concurrent read performance
	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	if err := db.AutoMigrate(&ChainRecord{}, &ExecutionRecord{}, &StepLog{}); err != nil {
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return &Store{db: db}, nil
}

// ── Chains ────────────────────────────────────────────────────────────────────

func (s *Store) CreateChain(r ChainRecord) error {
	return s.db.Create(&r).Error
}

func (s *Store) GetChain(id string) (*ChainRecord, error) {
	var r ChainRecord
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListChains() ([]ChainRecord, error) {
	var records []ChainRecord
	if err := s.db.Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) UpdateChain(r ChainRecord) error {
	return s.db.Save(&r).Error
}

func (s *Store) DeleteChain(id string) error {
	return s.db.Delete(&ChainRecord{}, "id = ?", id).Error
}

// ── Executions ────────────────────────────────────────────────────────────────

func (s *Store) CreateExecution(r ExecutionRecord) error {
	return s.db.Create(&r).Error
}

func (s *Store) GetExecution(id string) (*ExecutionRecord, error) {
	var r ExecutionRecord
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListExecutions(chainID string) ([]ExecutionRecord, error) {
	var records []ExecutionRecord
	q := s.db.Order("started_at DESC")
	if chainID != "" {
		q = q.Where("chain_id = ?", chainID)
	}
	if err := q.Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) UpdateExecutionStatus(id, status, errMsg string, finishedAt *time.Time) error {
	updates := map[string]interface{}{
		"status":      status,
		"error":       errMsg,
		"finished_at": finishedAt,
	}
	return s.db.Model(&ExecutionRecord{}).Where("id = ?", id).Updates(updates).Error
}

// ── Step Logs ─────────────────────────────────────────────────────────────────

func (s *Store) LogStep(executionID, stepID, status, stdout, stderr string, exitCode int, errMsg string, durationMs int64) error {
	log := &StepLog{
		ExecutionID: executionID,
		StepID:      stepID,
		Status:      status,
		Stdout:      stdout,
		Stderr:      stderr,
		ExitCode:    exitCode,
		Error:       errMsg,
		DurationMs:  durationMs,
	}
	// Upsert: update if already exists (step transitions from running → done/failed)
	var existing StepLog
	res := s.db.Where("execution_id = ? AND step_id = ?", executionID, stepID).First(&existing)
	if res.Error == nil {
		log.ID = existing.ID
		return s.db.Save(log).Error
	}
	return s.db.Create(log).Error
}

func (s *Store) GetStepLogs(executionID string) ([]StepLog, error) {
	var logs []StepLog
	if err := s.db.Where("execution_id = ?", executionID).
		Order("created_at ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *Store) CountStepLogs(executionID string, afterID uint) (int64, error) {
	var count int64
	return count, s.db.Model(&StepLog{}).
		Where("execution_id = ? AND id > ?", executionID, afterID).
		Count(&count).Error
}
