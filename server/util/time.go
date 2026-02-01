package util

import "time"

func CurrentLocalTime() time.Time {
	return time.Now().Local()
}

func CurrentLocalTimeRFC3339() string {
	return TimeToRFC3339(nil)
}

func TimeToRFC3339(t *time.Time) string {
	if t != nil {
		return t.Format(time.RFC3339)
	}

	return CurrentLocalTime().Format(time.RFC3339)
}
