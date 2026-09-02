package repositories

import "gorm.io/gorm"

// BaseRepository provides generic CRUD for any GORM model T, so per-resource
// repositories only need to add the queries that are actually special
// (filters, joins, etc.) instead of re-writing FindByID/Create/Update/Delete
// every time.
type BaseRepository[T any] struct {
	DB *gorm.DB
}

func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{DB: db}
}

func (r *BaseRepository[T]) FindByID(id interface{}) (*T, error) {
	var entity T
	if err := r.DB.First(&entity, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *BaseRepository[T]) FindAll(scopes ...func(*gorm.DB) *gorm.DB) ([]T, error) {
	var entities []T
	q := r.DB
	for _, s := range scopes {
		q = s(q)
	}
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (r *BaseRepository[T]) Count(scopes ...func(*gorm.DB) *gorm.DB) (int64, error) {
	var total int64
	var model T
	q := r.DB.Model(&model)
	for _, s := range scopes {
		q = s(q)
	}
	if err := q.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *BaseRepository[T]) Create(entity *T) error {
	return r.DB.Create(entity).Error
}

func (r *BaseRepository[T]) Update(entity *T) error {
	return r.DB.Save(entity).Error
}

func (r *BaseRepository[T]) Delete(id interface{}) error {
	var entity T
	return r.DB.Delete(&entity, "id = ?", id).Error
}
