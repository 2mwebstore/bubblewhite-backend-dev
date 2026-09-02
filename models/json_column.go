package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONColumn[T] lets any Go value (slices, structs, maps) be stored in a
// single MySQL JSON column via GORM, instead of hand-writing a Scan/Value
// pair for every field. Used for things like Product.Images ([]string),
// Product.Sizes ([]string), and Role.Permissions ([]string).
type JSONColumn[T any] struct {
	Data T
}

// NewJSON wraps a value as a JSONColumn, e.g. NewJSON([]string{"S","M","L"}).
func NewJSON[T any](v T) JSONColumn[T] {
	return JSONColumn[T]{Data: v}
}

func (j JSONColumn[T]) Value() (driver.Value, error) {
	return json.Marshal(j.Data)
}

func (j *JSONColumn[T]) Scan(value interface{}) error {
	if value == nil {
		var zero T
		j.Data = zero
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			bytes = []byte(s)
		} else {
			return errors.New("JSONColumn: unsupported scan source type")
		}
	}
	if len(bytes) == 0 {
		var zero T
		j.Data = zero
		return nil
	}
	return json.Unmarshal(bytes, &j.Data)
}

// MarshalJSON makes JSONColumn[T] serialize as the bare value in API
// responses, e.g. {"images": ["a.jpg","b.jpg"]} instead of {"images": {"Data": [...]}}.
func (j JSONColumn[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Data)
}

// UnmarshalJSON lets JSONColumn[T] be bound directly from request JSON bodies too.
func (j *JSONColumn[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.Data)
}

// GormDataType tells GORM which column type to use for AutoMigrate.
func (JSONColumn[T]) GormDataType() string {
	return "json"
}
