package storage

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseRetentionPeriod converts period strings to days
// Supports both predefined periods and flexible formats like "7 days", "1h", "2 weeks", etc.
func ParseRetentionPeriod(period string) (int, error) {
	if period == "" {
		return 0, errors.New("retention period cannot be empty")
	}

	normalized := strings.ToLower(strings.TrimSpace(period))

	// First try predefined periods for backward compatibility
	switch normalized {
	case "hourly":
		return 1, nil // Keep for at least 1 day for hourly backups
	case "daily":
		return 1, nil
	case "weekly":
		return 7, nil
	case "monthly":
		return 30, nil
	case "yearly":
		return 365, nil
	}

	// Parse flexible time format strings like "7 days", "1h", "2 weeks", etc.
	// Pattern matches: optional number, optional space, time unit
	re := regexp.MustCompile(`^(\d+)\s*(h|hr|hrs|hour|hours|d|day|days|w|week|weeks|m|month|months|y|year|years)$`)
	matches := re.FindStringSubmatch(normalized)

	if len(matches) != 3 {
		return 0, fmt.Errorf("unsupported retention period format '%s'. Supported formats: 'hourly', 'daily', 'weekly', 'monthly', 'yearly', "+
			"or '<number> <unit>' where unit can be h/hr/hour(s), d/day(s), w/week(s), m/month(s), y/year(s)", period)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number in retention period '%s': %w", period, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("retention period value must be positive, got %d", value)
	}

	unit := matches[2]

	// Convert to days based on unit
	switch unit {
	case "h", "hr", "hrs", "hour", "hours":
		// For hours, convert to days with proper rounding (minimum 1 day)
		days := (value + 23) / 24 // Round up to next day
		if days == 0 {
			days = 1 // Minimum 1 day for any hour-based retention
		}
		return days, nil
	case "d", "day", "days":
		return value, nil
	case "w", "week", "weeks":
		return value * 7, nil
	case "m", "month", "months":
		return value * 30, nil // Approximate month as 30 days
	case "y", "year", "years":
		return value * 365, nil
	default:
		return 0, fmt.Errorf("unsupported time unit '%s' in retention period '%s'", unit, period)
	}
}

// GetEffectiveRetentionDays returns the effective retention period in days
func (s *S3Storage) GetEffectiveRetentionDays() (int, error) {
	// If RetentionPeriod is set, parse it
	if s.RetentionPeriod != nil {
		return ParseRetentionPeriod(*s.RetentionPeriod)
	}

	// No retention period configured
	return 0, nil
}

// GetEffectiveRetentionDays returns the effective retention period in days for local storage
func (l *LocalStorage) GetEffectiveRetentionDays() (int, error) {
	// If RetentionPeriod is set, parse it
	if l.RetentionPeriod != nil {
		return ParseRetentionPeriod(*l.RetentionPeriod)
	}

	// No retention period configured
	return 0, nil
}

// IsRetentionConfigured checks if any retention policy is configured
func (s *S3Storage) IsRetentionConfigured() bool {
	return s.RetentionPeriod != nil || s.RetentionCount != nil
}

// IsRetentionConfigured checks if any retention policy is configured for local storage
func (l *LocalStorage) IsRetentionConfigured() bool {
	return l.RetentionPeriod != nil || l.RetentionCount != nil
}
