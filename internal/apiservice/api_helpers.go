package apiservice

import (
	"errors"
	"strings"

	"api-mock-system/internal/apirepo"
)

func normalizeMethod(m string) string {
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
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
		p += "/"
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
