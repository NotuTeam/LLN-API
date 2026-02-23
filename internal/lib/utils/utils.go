package utils

import (
	"regexp"
	"strings"
)

// GenerateSlug creates a URL-friendly slug from a string
func GenerateSlug(text string) string {
	slug := strings.ToLower(text)
	slug = strings.ReplaceAll(slug, " ", "-")
	
	// Remove special characters
	reg := regexp.MustCompile("[^a-z0-9-]+")
	slug = reg.ReplaceAllString(slug, "")
	
	// Remove multiple consecutive hyphens
	reg2 := regexp.MustCompile("-+")
	slug = reg2.ReplaceAllString(slug, "-")
	
	slug = strings.Trim(slug, "-")
	
	return slug
}

// Contains checks if a string exists in a slice
func Contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

// RemoveEmpty removes empty strings from a slice
func RemoveEmpty(slice []string) []string {
	var result []string
	for _, v := range slice {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}
