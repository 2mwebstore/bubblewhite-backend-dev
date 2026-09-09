package services

import (
	"errors"
	"log"
	"time"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
)

type AuditLogService struct {
	Logs  *repositories.AuditLogRepository
	GeoIP *IPIntelligenceService
}

func NewAuditLogService(logs *repositories.AuditLogRepository, geoIP *IPIntelligenceService) *AuditLogService {
	return &AuditLogService{Logs: logs, GeoIP: geoIP}
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
//
// The country/VPN/proxy enrichment happens AFTER the row is written,
// in a background goroutine — never inline here. A third-party API call
// on the hot path of every login, product update, and order placed in
// the app would add real, user-visible latency to all of them just to
// populate three log-table columns; doing it after the fact means the
// actual action being audited is never waiting on it.
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
		return
	}

	if s.GeoIP != nil && entry.IPAddress != "" {
		id := audit.ID
		ip := entry.IPAddress
		go func() {
			info := s.GeoIP.Lookup(ip)
			if info.Country == "" {
				return // nothing to add — lookup skipped/unavailable
			}
			if err := s.Logs.UpdateGeoInfo(id, info.Country); err != nil {
				log.Printf("audit log: failed to update geo info for entry %d: %v", id, err)
			}
		}()
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

var ErrInvalidRetentionPreset = errors.New("invalid retention preset")

// retentionCutoff turns one of the admin panel's four preset labels into
// an actual cutoff time — everything created before it gets deleted.
// Calendar-month-aligned (the start of the 1st of the relevant month, in
// Phnom Penh time — this app's own operating timezone, same choice as
// the backup scheduler) rather than a rolling N*30-day window, so "keep
// this month" means what it reads as regardless of which day of the
// month it's actually clicked.
func retentionCutoff(preset string) (time.Time, error) {
	loc, err := time.LoadLocation("Asia/Phnom_Penh")
	if err != nil {
		return time.Time{}, err
	}
	now := time.Now().In(loc)
	startOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	var monthsBack int
	switch preset {
	case "this_month":
		monthsBack = 0
	case "last_month":
		monthsBack = 1
	case "3_months":
		monthsBack = 3
	case "5_months":
		monthsBack = 5
	default:
		return time.Time{}, ErrInvalidRetentionPreset
	}
	return startOfThisMonth.AddDate(0, -monthsBack, 0), nil
}

// Cleanup deletes every entry of the given actor type older than the
// cutoff implied by preset — the "keep only the last N months" buttons
// on the admin panel's two log views. Returns the number of rows removed
// so the caller can show a concrete confirmation rather than a bare
// "done".
func (s *AuditLogService) Cleanup(actorType, preset string) (int64, error) {
	cutoff, err := retentionCutoff(preset)
	if err != nil {
		return 0, err
	}
	return s.Logs.DeleteOlderThan(actorType, cutoff)
}

// DeleteSelected removes specific entries by ID — the checkbox-based
// "select these rows, delete them" flow, distinct from Cleanup's preset-
// based bulk retention delete.
func (s *AuditLogService) DeleteSelected(actorType string, ids []uint) (int64, error) {
	return s.Logs.DeleteByIDs(actorType, ids)
}
