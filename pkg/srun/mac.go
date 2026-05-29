package srun

import "strings"

// NormalizeMAC lowercases and unifies separators so portal (aa-bb-...) and
// self-service (aa:bb:...) forms compare equal.
func NormalizeMAC(mac string) string {
	mac = strings.TrimSpace(strings.ToLower(mac))
	return strings.ReplaceAll(mac, "-", ":")
}

// MACEqual compares two MAC addresses for equivalence.
func MACEqual(a, b string) bool {
	return NormalizeMAC(a) == NormalizeMAC(b)
}
