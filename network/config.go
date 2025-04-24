package network

import "time"

type Config struct {
	receiveChanSize int32
	receiveTimeout  time.Duration

	sendChanSize int32
	sendTimeout  time.Duration
}
