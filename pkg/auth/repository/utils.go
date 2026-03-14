package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// isDuplicateKeyError checks if error is a duplicate key error
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "unique") ||
		   strings.Contains(errStr, "duplicate") ||
		   strings.Contains(errStr, "already exists")
}

// generateSlug creates a URL-friendly slug from a string
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = regexp.MustCompile(`[^a-z0-9\-_]`).ReplaceAllString(slug, "-")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if len(slug) == 0 {
		slug = "org"
	}
	if len(slug) > 50 {
		slug = slug[:50]
	}

	return slug
}

// generateUniqueSlug generates a unique slug by appending a number
func generateUniqueSlug(baseSlug string) string {
	return baseSlug + "-" + uuid.New().String()[:8]
}

// joinString joins strings with separator
func joinString(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// toJSON converts a value to JSON string
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// toJSONB converts a value to JSONB string (for PostgreSQL)
func toJSONB(v interface{}) string {
	return toJSON(v) // PostgreSQL handles JSONB conversion automatically
}

// fromJSON parses JSON string into a value
func fromJSON(jsonStr string, v interface{}) error {
	if jsonStr == "" {
		return nil
	}
	return json.Unmarshal([]byte(jsonStr), v)
}

// slugExists checks if a slug already exists
func (r *authRepository) slugExists(ctx context.Context, slug string) (bool, error) {
	query := r.formatQuery("SELECT COUNT(*) FROM organizations WHERE slug = ?")
	var count int
	err := r.db.GetContext(ctx, &count, query, slug)
	return count > 0, err
}