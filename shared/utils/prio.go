package utils

func PrioFromInt(priority int) string {
	if priority >= 5 {
		return "critical"
	} else if priority >= 4 {
		return "high"
	} else if priority >= 3 {
		return "normal"
	} else if priority >= 2 {
		return "low"
	} else {
		return "minimal"
	}
}
