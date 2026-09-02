package models

// Role groups a set of permission slugs under a name (e.g. "admin", "editor")
// and gets assigned to users. Permissions are stored as a JSON string array
// directly on the role for simple, single-query permission checks — no join
// table needed for the common case of "does this role have permission X".
type Role struct {
	ID          uint                 `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string               `json:"name" gorm:"type:varchar(100);not null"`
	Slug        string               `json:"slug" gorm:"type:varchar(100);uniqueIndex;not null"`
	Description string               `json:"description" gorm:"type:varchar(255)"`
	Permissions JSONColumn[[]string] `json:"permissions" gorm:"type:json"`
	IsSystem    bool                 `json:"isSystem" gorm:"default:false"` // built-in roles (admin) can't be deleted
}

// HasPermission reports whether this role grants the given permission slug.
func (r Role) HasPermission(slug string) bool {
	for _, p := range r.Permissions.Data {
		if p == slug || p == "*" {
			return true
		}
	}
	return false
}
