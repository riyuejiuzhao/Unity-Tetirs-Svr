package network

import "time"

type Config struct {
	ReceiveChanSize int32
	ReceiveTimeout  time.Duration

	SendChanSize int32
	SendTimeout  time.Duration
}
