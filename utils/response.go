package utils

import "github.com/gin-gonic/gin"

// Envelope is the shape every API response takes:
//
//	{ "success": true,  "data": ... }
//	{ "success": false, "error": "message", "errors": {...} }
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
	Error   string      `json:"error,omitempty"`
	Errors  interface{} `json:"errors,omitempty"`
}

// OK sends a 200 with the given data.
func OK(c *gin.Context, data interface{}) {
	c.JSON(200, Envelope{Success: true, Data: data})
}

// OKWithMeta sends a 200 with data plus a meta block (e.g. pagination info).
func OKWithMeta(c *gin.Context, data interface{}, meta interface{}) {
	c.JSON(200, Envelope{Success: true, Data: data, Meta: meta})
}

// Created sends a 201 with the given data.
func Created(c *gin.Context, data interface{}) {
	c.JSON(201, Envelope{Success: true, Data: data})
}

// NoContent sends a 204 (used for deletes).
func NoContent(c *gin.Context) {
	c.Status(204)
}

// Fail sends an error envelope with the given HTTP status and message.
func Fail(c *gin.Context, status int, message string) {
	c.JSON(status, Envelope{Success: false, Error: message})
}

// FailWithErrors sends a 422 with a field-level error map (see validator.go).
func FailWithErrors(c *gin.Context, errors interface{}) {
	c.JSON(422, Envelope{Success: false, Error: "validation failed", Errors: errors})
}

// BadRequest sends a 400.
func BadRequest(c *gin.Context, message string) { Fail(c, 400, message) }

// Unauthorized sends a 401.
func Unauthorized(c *gin.Context, message string) { Fail(c, 401, message) }

// Forbidden sends a 403.
func Forbidden(c *gin.Context, message string) { Fail(c, 403, message) }

// NotFound sends a 404.
func NotFound(c *gin.Context, message string) { Fail(c, 404, message) }

// InternalError sends a 500.
func InternalError(c *gin.Context, message string) { Fail(c, 500, message) }
