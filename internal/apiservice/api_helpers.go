package apiservice

import (
	"errors"
	"strings"

	"api-mock-system/internal/apirepo"
)

// normalizeMethod upper-cases and validates the HTTP method. It returns the
// canonical uppercase form (matching net/http's http.Method* constants and the
// raw value of http.Request.Method) so a stored method compares directly with an
// inbound request method. The mock resolver and the OpenAPI builder both assume
// uppercase; storing lowercase here made published POSTs (and every other
// method) unreachable — the stored value never matched the inbound method.
func normalizeMethod(m string) string {
	m = strings.ToUpper(strings.TrimSpace(m))
	switch m {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return m
	}
	return ""
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	return p
}

func validStatus(s string) bool {
	switch s {
	case "designing", "published", "deprecated":
		return true
	}
	return false
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func mapErr(err error) error {
	if errors.Is(err, apirepo.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
