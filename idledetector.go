package main

import "time"

type idledetector struct {
	duration time.Duration
	timer    *time.Timer
}

func (id *idledetector) NotIdle() {
	if id.timer != nil {
		id.timer.Stop()
		id.timer.Reset(id.duration)
	}
}

func (id *idledetector) Wait() {
	if id.timer == nil {
		id.timer = time.NewTimer(id.duration)
	}
	<-id.timer.C
}
