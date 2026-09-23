package config

// emptyOr 在 value 为零值时返回 fallback。
func emptyOr[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}
	return value
}
