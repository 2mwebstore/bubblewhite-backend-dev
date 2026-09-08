package services

import (
	"log"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
)

type AuditLogService struct {
	Logs *repositories.AuditLogRepository
}

func NewAuditLogService(logs *repositories.AuditLogRepository) *AuditLogService {
	return &AuditLogService{Logs: logs}
}

// LogEntry is what every call site fills in — ResourceID/ResourceLabel are
// plain strings here (empty means "not applicable"), converted to the
// model's nullable *string fields internally, so callers never have to
// deal with pointer plumbing just to record an entry.
type LogEntry struct {
	ActorType     string // "admin" | "customer"
	ActorID       uint
	ActorName     string
	Action        string
	Resource      string
	ResourceID    string
	ResourceLabel string
	Description   string
	IPAddress     string
	UserAgent     string
}

// Log records one audit entry. Deliberately swallows its own errors
// (logged, never returned) — same philosophy as
// telegram_service.go's sendTelegramMessage: a failure to WRITE an audit
// record must never fail or block the actual action being audited. An
// admin losing the ability to update a product because the audit table
// had a hiccup would be a far worse outcome than one missing log entry.
func (s *AuditLogService) Log(entry LogEntry) {
	audit := &models.AuditLog{
		ActorType:   entry.ActorType,
		ActorID:     entry.ActorID,
		ActorName:   entry.ActorName,
		Action:      entry.Action,
		Resource:    entry.Resource,
		Description: entry.Description,
		IPAddress:   entry.IPAddress,
		UserAgent:   entry.UserAgent,
	}
	if entry.ResourceID != "" {
		audit.ResourceID = &entry.ResourceID
	}
	if entry.ResourceLabel != "" {
		audit.ResourceLabel = &entry.ResourceLabel
	}
	if err := s.Logs.Create(audit); err != nil {
		log.Printf("audit log: failed to record entry (actor=%s:%d action=%s resource=%s): %v", entry.ActorType, entry.ActorID, entry.Action, entry.Resource, err)
	}
}

// List returns a filtered, paginated page of audit entries — the actual
// query behind both of the admin panel's log views. ActorType on filter
// is what the caller (AdminAuditLogController) uses to keep those two
// views strictly separate.
func (s *AuditLogService) List(page utils.PageParams, filter repositories.AuditLogFilter) ([]models.AuditLog, int64, error) {
	scope := filter.Scope()
	total, err := s.Logs.Count(scope)
	if err != nil {
		return nil, 0, err
	}
	logs, err := s.Logs.FindAll(scope, page.Scope(), func(db *gorm.DB) *gorm.DB { return db.Order("created_at DESC") })
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// FilterOptions returns the distinct action/resource values currently
// available for an actor type, for populating the admin panel's filter
// dropdowns — see AuditLogRepository.DistinctActions/DistinctResources
// for why these are queried live rather than hardcoded.
type FilterOptions struct {
	Actions   []string `json:"actions"`
	Resources []string `json:"resources"`
}

func (s *AuditLogService) FilterOptions(actorType string) (*FilterOptions, error) {
	actions, err := s.Logs.DistinctActions(actorType)
	if err != nil {
		return nil, err
	}
	resources, err := s.Logs.DistinctResources(actorType)
	if err != nil {
		return nil, err
	}
	return &FilterOptions{Actions: actions, Resources: resources}, nil
}
