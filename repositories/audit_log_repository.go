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
	// DateFrom/DateTo come from the admin panel's date-range picker as
	// plain "YYYY-MM-DD" strings (this component intentionally works in
	// whole days, not specific times — see DateRangePicker.vue). Also
	// still accepts a full "YYYY-MM-DDTHH:mm" datetime-local value for
	// backward compatibility with anything else that might send one.
	DateFrom string
	DateTo   string
}

// normalizeFrom converts a DateFrom value into the exact instant MySQL
// should compare against: a plain date becomes that day's midnight
// (already correct as the literal string — MySQL treats "2026-09-08" as
// "2026-09-08 00:00:00" in a DATETIME comparison), while a datetime-local
// value just needs its "T" swapped for the space MySQL expects.
func normalizeFrom(s string) string {
	return strings.Replace(s, "T", " ", 1)
}

// normalizeTo converts a DateTo value into the exact instant MySQL should
// compare against — critically, NOT just normalizeFrom: a plain date
// like "2026-09-08" would otherwise be compared as that day's midnight,
// which excludes the entire rest of that day from the range. A plain
// date is extended to 23:59:59 so "to 8 September" actually includes all
// of the 8th; a value that already carries a time component (contains a
// "T") is left as the exact instant it specifies, unchanged.
func normalizeTo(s string) string {
	if strings.Contains(s, "T") {
		return strings.Replace(s, "T", " ", 1)
	}
	return s + " 23:59:59"
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
			db = db.Where("created_at >= ?", normalizeFrom(f.DateFrom))
		}
		if f.DateTo != "" {
			db = db.Where("created_at <= ?", normalizeTo(f.DateTo))
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

// DeleteByIDs removes specific entries by ID — the checkbox-based "select
// these rows, delete them" flow on the admin panel, distinct from
// DeleteOlderThan's "keep only the last N months" bulk cleanup. Still
// scoped by ActorType, same reasoning as everywhere else on this table:
// a request against the staff log view can never delete a customer
// entry just because its ID happened to be guessed or reused, even if
// somehow included in the request.
func (r *AuditLogRepository) DeleteByIDs(actorType string, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.DB.Where("actor_type = ? AND id IN ?", actorType, ids).Delete(&models.AuditLog{})
	return result.RowsAffected, result.Error
}

// UpdateGeoInfo patches just the country field on one already-created
// entry — called from a background goroutine once the IP geolocation
// lookup for that entry's IP completes (see AuditLogService.Log), never
// as part of creating the row itself.
func (r *AuditLogRepository) UpdateGeoInfo(id uint, country string) error {
	return r.DB.Model(&models.AuditLog{}).Where("id = ?", id).Update("country", country).Error
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
