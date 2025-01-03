package models

func CopySlice[T Copyable[T]](slice []T) []T {
	newCopy := make([]T, len(slice))
	for i, elem := range slice {
		newCopy[i] = elem.Copy()
	}
	return newCopy
}

func NormalizeSlice[T Normalizable](slice []T) {
	for _, elem := range slice {
		elem.Normalize()
	}
}

func ValidateSlice[T Validatable](slice []T) error {
	for _, elem := range slice {
		if err := elem.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Helper function for copying or getting zero value
func copyOrZero[T any](v *T) T {
	var zero T // Create zero value
	if v == nil {
		return zero
	}
	return *v
}
