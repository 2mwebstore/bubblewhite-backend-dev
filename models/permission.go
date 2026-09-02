package models

// Permission is a single grantable action, e.g. "product.create".
// The catalog of permissions is fixed in code (see seed.PermissionCatalog)
// and synced into this table on boot; roles reference permissions by slug.
type Permission struct {
	ID          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Slug        string `json:"slug" gorm:"type:varchar(100);uniqueIndex;not null"`
	Description string `json:"description" gorm:"type:varchar(255)"`
	Group       string `json:"group" gorm:"type:varchar(50);index"` // e.g. "product", "category", "settings", "user"
}
