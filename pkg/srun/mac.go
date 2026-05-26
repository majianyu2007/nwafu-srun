package srun

import "strings"

// normalizeMAC lowercases and unifies separators so portal (aa-bb-...) and
// self-service (aa:bb:...) forms compare equal.
func normalizeMAC(mac string) string {
	mac = strings.TrimSpace(strings.ToLower(mac))
	return strings.ReplaceAll(mac, "-", ":")
}
