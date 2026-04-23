package config

import (
	"strings"
)

// ReverseProxyMediaBase returns the URL base for pserver's --reverseproxymedia flag
// (no trailing slash). For edgeScheme "dual", HTTPS is the canonical URL advertised to clients.
func ReverseProxyMediaBase(edgeScheme, mediaHost string) string {
	host := strings.TrimSpace(mediaHost)
	scheme := "https"
	switch strings.ToLower(strings.TrimSpace(edgeScheme)) {
	case "http":
		scheme = "http"
	case "https", "dual":
		scheme = "https"
	default:
		scheme = "https"
	}
	return scheme + "://" + host
}
