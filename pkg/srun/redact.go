package srun

import "regexp"

var (
	redactQueryParamRe = regexp.MustCompile(`(?i)([?&](?:info|chksum|password)=)[^&]*`)
	redactJSONFieldRe  = regexp.MustCompile(`(?i)("(?:info|password|chksum)"\s*:\s*")([^"]*)"`)
)

// redactSensitive masks credential-bearing portal fields in URLs and JSON bodies
// before writing them to verbose logs.
func redactSensitive(s string) string {
	if s == "" {
		return s
	}
	s = redactQueryParamRe.ReplaceAllString(s, `${1}<REDACTED>`)
	s = redactJSONFieldRe.ReplaceAllString(s, `${1}<REDACTED>"`)
	return s
}
