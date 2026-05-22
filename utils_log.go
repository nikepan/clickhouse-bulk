package main

import (
	"fmt"
	"strings"
)

const logTruncateDefault = 256

// redactURLParams masks sensitive query-string keys (password, tokens) for logs.
func redactURLParams(params string) string {
	if params == "" {
		return ""
	}
	parts := strings.Split(params, "&")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		key, _, ok := strings.Cut(p, "=")
		if !ok {
			out = append(out, p)
			continue
		}
		if isSensitiveParamKey(key) {
			out = append(out, key+"=***")
		} else {
			out = append(out, p)
		}
	}
	return strings.Join(out, "&")
}

func isSensitiveParamKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "password", "pass", "pwd", "secret", "token", "access_key", "api_key", "key":
		return true
	default:
		return false
	}
}

// logInsertMeta returns a safe summary for INSERT logging (no row payload).
func logInsertMeta(params, content string) string {
	rows := 0
	if content != "" {
		rows = strings.Count(content, "\n") + 1
	}
	return fmt.Sprintf("params=%q rows=%d content_bytes=%d", redactURLParams(params), rows, len(content))
}

// logTruncate limits string length in log lines.
func logTruncate(s string, max int) string {
	if max <= 0 {
		max = logTruncateDefault
	}
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
