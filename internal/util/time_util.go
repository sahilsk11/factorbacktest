package util

import (
	"time"
)

const layout = "2006-01-02"

func NewDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func DateLte(t1, t2 time.Time) bool {
	return t1.Before(t2) || t1.Format(layout) == t2.Format(layout)
}
