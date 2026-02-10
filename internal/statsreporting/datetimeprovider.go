package statsreporting

import "time"

type SystemDateTimeProvider struct{}

func NewSystemDateTimeProvider() *SystemDateTimeProvider {
	return &SystemDateTimeProvider{}
}

func (r *SystemDateTimeProvider) Now() time.Time {
	return time.Now()
}
