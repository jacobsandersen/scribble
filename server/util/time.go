package util

import "time"

func CurrentTimeRFC3339() string {
	return time.Now().Local().Format(time.RFC3339)
}
