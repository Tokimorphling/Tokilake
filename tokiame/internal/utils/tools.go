package utils

func SliceToMap[T any](slice []T, idFunc func(T) string) map[string]T {
	result := make(map[string]T)
	for _, item := range slice {
		key := idFunc(item)
		result[key] = item
	}
	return result
}
