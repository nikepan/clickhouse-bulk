package main

import (
	"bytes"
	"mime"
	"net/url"
	"regexp"
	"strings"
)

var opaqueFormatInQuery = regexp.MustCompile(`(?i)\bformat\s+(Native|RowBinary|Parquet|Arrow|ArrowStream|ORC|Protobuf)\b`)

// shouldOpaqueInsert reports whether an INSERT should bypass collector batching.
func shouldOpaqueInsert(forceAll bool, contentType, params string, body []byte) bool {
	if forceAll {
		return isInsertParamsOrBody(params, body)
	}
	if isOctetStreamContentType(contentType) && isInsertParamsOrBody(params, body) {
		return true
	}
	q := insertQueryString(params, body)
	return q != "" && opaqueFormatInQuery.MatchString(q)
}

func isInsertParamsOrBody(params string, body []byte) bool {
	if paramsIndicateInsert(params) {
		return true
	}
	if len(body) == 0 {
		return false
	}
	// Text INSERT with query in body (before optional binary payload).
	prefix := body
	if len(prefix) > 256 {
		prefix = prefix[:256]
	}
	return HasPrefix(strings.TrimSpace(string(prefix)), "insert")
}

func paramsIndicateInsert(params string) bool {
	q := queryFromParams(params)
	return q != "" && HasPrefix(strings.TrimSpace(q), "insert")
}

func queryFromParams(params string) string {
	for _, p := range strings.Split(params, "&") {
		if !HasPrefix(p, "query=") {
			continue
		}
		raw := p[6:]
		if j := strings.Index(raw, "&"); j >= 0 {
			raw = raw[:j]
		}
		q, err := url.QueryUnescape(raw)
		if err != nil {
			return raw
		}
		return q
	}
	return ""
}

func insertQueryString(params string, body []byte) string {
	if q := queryFromParams(params); q != "" {
		return q
	}
	if len(body) == 0 {
		return ""
	}
	if idx := bytes.IndexByte(body, '\n'); idx >= 0 {
		return strings.TrimSpace(string(body[:idx]))
	}
	return strings.TrimSpace(string(body))
}

func isOctetStreamContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mt = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	return strings.EqualFold(mt, "application/octet-stream")
}

// outboundContentType picks the Content-Type for the ClickHouse POST.
func outboundContentType(clientCT, insertQuery string) string {
	if clientCT != "" {
		mt, _, err := mime.ParseMediaType(clientCT)
		if err == nil && mt != "" {
			return mt
		}
	}
	if opaqueFormatInQuery.MatchString(insertQuery) {
		return "application/octet-stream"
	}
	return "text/plain"
}
