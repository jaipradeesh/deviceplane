package supervisor

import (
	"time"
)

const (
	defaultTickerFrequency  = 3 * time.Second
	fullDataTickerFrequency = 60 * time.Second
)
