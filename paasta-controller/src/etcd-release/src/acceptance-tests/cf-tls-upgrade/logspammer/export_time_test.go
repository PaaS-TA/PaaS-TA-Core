package logspammer

import "time"

func SetTimeNow(f func() time.Time) {
	timeNow = f
}

func ResetTimeNow() {
	timeNow = time.Now
}
