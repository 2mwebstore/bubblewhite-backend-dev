package repositories

import (
	"strings"
	"time"

	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type AuditLogRepository struct {
	*BaseRepository[models.AuditLog]
}

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{BaseRepository: NewBaseRepository[models.AuditLog](db)}
}

// AuditLogFilter is the audit log list's filter set — same
// struct-plus-Scope() pattern as OrderFilter/ProductFilter. ActorType is
// what actually separates the admin panel's two log views (staff vs
// customer) — see AdminAuditLogController.List, which always sets this
// explicitly per route rather than leaving it optional, so the two views
// can never accidentally bleed into each other.
type AuditLogFilter struct {
	ActorType string // "admin" | "customer" — always set by the controller, not user-supplied
	ActorID   string // optional: one specific actor's history (e.g. embedded in a customer's own detail page)
	Action    string
	Resource  string
	Search    string // matches ActorName, Description, or ResourceLabel
	// DateFrom/DateTo come from the admin panel's datetime-range picker
	// (<input type="datetime-local">), so they arrive as
	// "YYYY-MM-DDTHH:mm" — normalizeDateTime below converts that "T" to
	// the space MySQL's DATETIME comparison expects. Still accepts a
	// plain "YYYY-MM-DD" too (treated as midnight that day), so an older
	// caller passing just a date doesn't break.
	DateFrom string
	DateTo   string
}

// normalizeDateTime converts an HTML datetime-local value
// ("2026-09-08T14:30") into the "2026-09-08 14:30" form MySQL expects —
// or passes a plain date straight through unchanged, since
// "2026-09-08" alone is already valid as the start of that day in a
// MySQL comparison.
func normalizeDateTime(s string) string {
	return strings.Replace(s, "T", " ", 1)
}

func (f AuditLogFilter) Scope() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if f.ActorType != "" {
			db = db.Where("actor_type = ?", f.ActorType)
		}
		if f.ActorID != "" {
			db = db.Where("actor_id = ?", f.ActorID)
		}
		if f.Action != "" {
			db = db.Where("action = ?", f.Action)
		}
		if f.Resource != "" {
			db = db.Where("resource = ?", f.Resource)
		}
		if f.Search != "" {
			like := "%" + f.Search + "%"
			db = db.Where("actor_name LIKE ? OR description LIKE ? OR resource_label LIKE ?", like, like, like)
		}
		if f.DateFrom != "" {
			db = db.Where("created_at >= ?", normalizeDateTime(f.DateFrom))
		}
		if f.DateTo != "" {
			db = db.Where("created_at <= ?", normalizeDateTime(f.DateTo))
		}
		return db
	}
}

// DeleteOlderThan removes every entry of the given actor type created
// before cutoff — the actual operation behind the admin panel's "keep
// only the last N months" cleanup buttons (see AdminAuditLogController's
// own doc comment for how each preset's cutoff is computed). Scoped by
// ActorType the same way every other query on this table is, so running
// this from the staff log view can never touch customer entries and vice
// versa. Returns the number of rows actually removed so the admin gets a
// concrete "deleted 1,204 entries" confirmation, not just a bare success.
func (r *AuditLogRepository) DeleteOlderThan(actorType string, cutoff time.Time) (int64, error) {
	result := r.DB.Where("actor_type = ? AND created_at < ?", actorType, cutoff).Delete(&models.AuditLog{})
	return result.RowsAffected, result.Error
}

// DistinctActions/DistinctResources power the filter dropdowns on the
// admin panel's log views — populated from whatever values actually
// exist for that actor type, rather than a hardcoded list that would
// drift out of sync with the real set of actions/resources being logged
// as new call sites are added over time.
func (r *AuditLogRepository) DistinctActions(actorType string) ([]string, error) {
	var actions []string
	err := r.DB.Model(&models.AuditLog{}).
		Where("actor_type = ?", actorType).
		Distinct().Pluck("action", &actions).Error
	return actions, err
}

func (r *AuditLogRepository) DistinctResources(actorType string) ([]string, error) {
	var resources []string
	err := r.DB.Model(&models.AuditLog{}).
		Where("actor_type = ?", actorType).
		Distinct().Pluck("resource", &resources).Error
	return resources, err
}
