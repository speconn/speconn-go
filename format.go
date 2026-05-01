package speconn

import "strings"

// ExtractFormat extracts the format name from a MIME type or content-type header.
// This is Speconn's responsibility — Specodec only knows "json", "msgpack", "gron".
//
//	"application/json"            → "json"
//	"application/msgpack"         → "msgpack"
//	"application/connect+json"    → "json"
//	"application/connect+msgpack" → "msgpack"
func ExtractFormat(contentType string) string {
	if strings.Contains(contentType, "msgpack") {
		return "msgpack"
	}
	return "json"
}

// FormatToMime maps a format name back to its canonical MIME type.
// isStream=true produces "application/connect+json" style types.
func FormatToMime(format string, isStream bool) string {
	var base string
	if format == "msgpack" {
		base = "application/msgpack"
	} else {
		base = "application/json"
	}
	if isStream {
		return strings.Replace(base, "application/", "application/connect+", 1)
	}
	return base
}
