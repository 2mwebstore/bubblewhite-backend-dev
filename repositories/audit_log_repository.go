package repositories

import (
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
	DateFrom  string // "YYYY-MM-DD", inclusive
	DateTo    string // "YYYY-MM-DD", inclusive
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
			db = db.Where("created_at >= ?", f.DateFrom+" 00:00:00")
		}
		if f.DateTo != "" {
			db = db.Where("created_at <= ?", f.DateTo+" 23:59:59")
		}
		return db
	}
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
