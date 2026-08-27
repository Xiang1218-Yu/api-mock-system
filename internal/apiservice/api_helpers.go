package apiservice

import (
	"errors"

	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/pathmatch"
)

func normalizeMethod(m string) string {
	switch m {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return m
	}
	return ""
}

// normalizePath canonicalizes a path for storage, dedup, and matching. It is a
// thin wrapper over pathmatch.Normalize so every layer applies the exact same
// rule — see that function for the shape contract (no trailing slash, root "/").
func normalizePath(p string) string {
	return pathmatch.Normalize(p)
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
