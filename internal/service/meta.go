package service

import (
	"strconv"
	"time"
)

const metaLastChecked = "last_checked_at"

// LastChecked returns when the store was last polled, or nil if never.
func (s *Service) LastChecked() (*time.Time, error) {
	v, ok, err := s.st.GetMeta(metaLastChecked)
	if err != nil || !ok {
		return nil, err
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, nil
	}
	t := time.Unix(n, 0)
	return &t, nil
}
