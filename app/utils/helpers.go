package utils
import (
	"fmt"
	"strings"
	"time"
)

// GenerateIDWithURL creates a URL-safe ID from name and URL combination
func GenerateIDWithURL(name, url string) string {
	combined := name + "-" + url
	id := ""
	for _, c := range combined {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			id += string(c)
		} else if c == ' ' || c == '-' || c == '_' || c == '/' || c == ':' || c == '.' {
			id += "-"
		}
	}
	// Trim multiple dashes and trailing dashes
	result := ""
	prevDash := false
	for _, c := range id {
		if c == '-' {
			if !prevDash {
				result += string(c)
			}
			prevDash = true
		} else {
			result += string(c)
			prevDash = false
		}
	}
	// Trim trailing dash
	for len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	return result
}

func FormatDurationDHm(d time.Duration) string {
	if d < 0 {
		return "-"
	}

	totalMinutes := int(d.Minutes())

	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60

	var parts []string

	if days > 0 {
		unit := "day"
		if days > 1 {
			unit = "days"
		}
		parts = append(parts, fmt.Sprintf("%d %s", days, unit))
	}

	if hours > 0 {
		unit := "hour"
		if hours > 1 {
			unit = "hours"
		}
		parts = append(parts, fmt.Sprintf("%d %s", hours, unit))
	}

	if minutes > 0 {
		unit := "min"
		if minutes > 1 {
			unit = "mins"
		}
		parts = append(parts, fmt.Sprintf("%d %s", minutes, unit))
	}

	if len(parts) == 0 {
		return "0 min"
	}

	return strings.Join(parts, " ")
}
