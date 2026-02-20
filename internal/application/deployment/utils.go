package deployment

import "strings"

func convertVolumeProtocol(v string) string {
	return strings.Replace(v, "volumes://", "./", 1)
}

func extractNamedVolume(v string) string {
	if strings.HasPrefix(v, "./") || strings.HasPrefix(v, "/") {
		return ""
	}
	parts := strings.SplitN(v, ":", 2)
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
