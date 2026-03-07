package session

import (
	"strings"
)

type DeviceInfo struct {
	DeviceType string
	DeviceName string
	Browser    string
	OS         string
}

// ParseUserAgent extracts device information from a User-Agent string.
func ParseUserAgent(ua string) DeviceInfo {
	if ua == "" {
		return DeviceInfo{DeviceType: "unknown"}
	}

	info := DeviceInfo{
		DeviceType: "desktop",
		OS:         parseOS(ua),
		Browser:    parseBrowser(ua),
	}

	// Detect mobile devices
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "mobile") ||
		strings.Contains(uaLower, "android") && !strings.Contains(uaLower, "tablet") ||
		strings.Contains(uaLower, "iphone") ||
		strings.Contains(uaLower, "ipod") {
		info.DeviceType = "mobile"
	} else if strings.Contains(uaLower, "tablet") ||
		strings.Contains(uaLower, "ipad") {
		info.DeviceType = "tablet"
	}

	// Use OS as device name
	info.DeviceName = info.OS

	return info
}

func parseOS(ua string) string {
	switch {
	case strings.Contains(ua, "Windows NT 10"):
		return "Windows 10"
	case strings.Contains(ua, "Windows NT 6.3"):
		return "Windows 8.1"
	case strings.Contains(ua, "Windows NT 6.2"):
		return "Windows 8"
	case strings.Contains(ua, "Windows NT 6.1"):
		return "Windows 7"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Mac OS X"):
		return "macOS"
	case strings.Contains(ua, "iPhone"):
		return "iOS"
	case strings.Contains(ua, "iPad"):
		return "iPadOS"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	case strings.Contains(ua, "CrOS"):
		return "Chrome OS"
	default:
		return "Unknown"
	}
}

func parseBrowser(ua string) string {
	// Order matters - check specific browsers before generic ones
	switch {
	case strings.Contains(ua, "Edg/"):
		return extractBrowserVersion(ua, "Edg/", "Edge")
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		return extractBrowserVersion(ua, "OPR/", "Opera")
	case strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Chromium"):
		return extractBrowserVersion(ua, "Chrome/", "Chrome")
	case strings.Contains(ua, "Safari/") && strings.Contains(ua, "Version/"):
		return extractBrowserVersion(ua, "Version/", "Safari")
	case strings.Contains(ua, "Firefox/"):
		return extractBrowserVersion(ua, "Firefox/", "Firefox")
	default:
		return "Unknown"
	}
}

func extractBrowserVersion(ua, marker, name string) string {
	idx := strings.Index(ua, marker)
	if idx == -1 {
		return name
	}
	start := idx + len(marker)
	end := start
	for end < len(ua) && ua[end] != ' ' && ua[end] != '.' {
		end++
	}
	if end > start {
		return name + " " + ua[start:end]
	}
	return name
}
