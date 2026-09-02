package utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func init() {
	// By default, validator.FieldError.Field() returns the Go struct field
	// name (e.g. "CompareAt", "ID") — but every request/response body in
	// this API is camelCase JSON ("compareAt", "id", "roleId", ...). Without
	// this, ValidateStruct's error map keys wouldn't match what the
	// frontend actually looks for: a snake_case guess derived from the Go
	// name ("compare_at") for multi-word fields, or a mangled acronym
	// ("i_d" for "ID") for single-word ones. Registering the JSON tag name
	// here makes Field() return exactly the JSON key the client sent, so
	// the error map lines up with the request body with no guessing.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}

// ValidateStruct runs struct tag validation and, on failure, returns a
// field-name -> human message map suitable for FailWithErrors(). The map
// keys are the struct's actual JSON field names (see RegisterTagNameFunc
// above), so they match the request body exactly — no case-conversion
// guessing that could drift out of sync with the JSON tags.
// Returns (nil, nil) when validation passes.
func ValidateStruct(s interface{}) (map[string]string, error) {
	if err := validate.Struct(s); err != nil {
		verrs, ok := err.(validator.ValidationErrors)
		if !ok {
			return nil, err
		}
		out := make(map[string]string, len(verrs))
		for _, fe := range verrs {
			out[fe.Field()] = fieldErrorMessage(fe)
		}
		return out, nil
	}
	return nil, nil
}

func fieldErrorMessage(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fe.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
