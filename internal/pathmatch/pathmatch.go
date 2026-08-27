// Package pathmatch matches concrete request paths against parameterized
// API path patterns (e.g. "/users/:id" matches "/users/42"). It is pure logic,
// no I/O — the caller feeds it the candidate patterns and asks for a match.
package pathmatch

// Normalize canonicalizes a path so storage, dedup, and matching all agree.
//
// A trailing slash is removed (except for the root "/"), and a leading slash is
// guaranteed. Thus "/users", "/users/", and "users" all become "/users", and
// "/", "", and "/" all become "/". This is the single authority for path shape:
// every layer that persists or looks up a path must route it through here, so a
// path defined with a trailing slash and the same path without one cannot be
// stored as two resources.
func Normalize(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	// Drop trailing slashes so "/users/" == "/users"; keep the root "/" intact.
	for len(p) > 1 && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}
	return p
}

// Match reports whether pattern matches path, and if so returns the path
// parameters keyed by name. Pattern segments starting with ':' are parameters
// that match any single segment.
//
// Both sides are Normalize'd first, so a trailing slash on either the stored
// pattern or the inbound request never changes the result — "/users/:id/"
// matches "/users/42", "/users/42/", and "users/42" identically.
//
// Wildcard suffix ":path" (Gin's catch-all) is not supported here because the
// mock layer matches one API at a time and patterns are explicit.
func Match(pattern, path string) (map[string]string, bool) {
	pSeg := splitSlash(Normalize(pattern))
	aSeg := splitSlash(Normalize(path))
	if len(pSeg) != len(aSeg) {
		return nil, false
	}
	params := map[string]string{}
	for i, seg := range pSeg {
		if len(seg) > 0 && seg[0] == ':' {
			params[seg[1:]] = aSeg[i]
			continue
		}
		if seg != aSeg[i] {
			return nil, false
		}
	}
	return params, true
}

// splitSlash breaks a path into segments, ignoring a leading slash so that
// "/users/42" and "users/42" both yield ["users","42"].
func splitSlash(p string) []string {
	if p == "" {
		return nil
	}
	if p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return nil
	}
	// Manual split to avoid allocating for the common single-segment case.
	var out []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			out = append(out, p[start:i])
			start = i + 1
		}
	}
	out = append(out, p[start:])
	return out
}
