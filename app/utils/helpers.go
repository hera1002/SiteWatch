package utils

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
