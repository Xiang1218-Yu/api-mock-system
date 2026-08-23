// Package httpx contains small helpers shared by every HTTP handler: a uniform
// JSON envelope and a shortcut for writing error responses. Handlers stay thin
// because they all funnel through these two functions, so the response shape is
// consistent across the whole API.
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope is the standard response body: data on success, message+errors on failure.
type Envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// OK writes a 200 response carrying data.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
}

// Created writes a 201 response carrying the newly-created resource.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data})
}

// Error writes the given status with a message; extra details go in Errors.
func Error(c *gin.Context, status int, msg string, details ...any) {
	env := Envelope{Success: false, Message: msg}
	if len(details) > 0 {
		env.Errors = details[0]
	}
	c.AbortWithStatusJSON(status, env)
}

// Page wraps a list payload with pagination metadata.
type Page struct {
	Items any   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// PageOf constructs a Page and writes it as a 200.
func PageOf(c *gin.Context, items any, total int64, page, size int) {
	OK(c, Page{Items: items, Total: total, Page: page, Size: size})
}

// Bind extracts JSON into dst and writes a 400 on failure. Returns false if the
// caller should abort (the response has already been written).
func Bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		Error(c, http.StatusBadRequest, "invalid request body", err.Error())
		return false
	}
	return true
}

// APIError lets services signal an HTTP status alongside a message, so handlers
// can map domain failures to status codes without a giant switch.
type APIError struct {
	Status  int
	Message string
}

// Error implements error.
func (e *APIError) Error() string { return e.Message }

// NewError builds an APIError.
func NewError(status int, msg string) *APIError { return &APIError{Status: status, Message: msg} }

// AsAPIError unwraps err into an *APIError if it is one, else returns nil.
func AsAPIError(err error) *APIError {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}
