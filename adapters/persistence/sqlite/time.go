package sqlite

import "time"

const storedTimeLayout = "2006-01-02T15:04:05.000000000Z"

func encodeStoredTime(value time.Time) (string, error) {
	if value.IsZero() {
		return "", nil
	}
	normalized := value.UTC()
	if normalized.Year() < 1 || normalized.Year() > 9999 {
		return "", ErrInvalidInput
	}
	return normalized.Format(storedTimeLayout), nil
}

func decodeStoredTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(storedTimeLayout, value)
	if err != nil {
		return time.Time{}, ErrStorage
	}
	return parsed, nil
}
