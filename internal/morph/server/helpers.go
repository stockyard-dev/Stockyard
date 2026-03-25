package server

import "time"

func timeNowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

func timeNowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
