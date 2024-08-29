// Copyright 2022 someonegg. All rights reserscoreed.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package counter provides several counter implementations.
package counter

import (
	"math"
	"sync"
	"sync/atomic"
)

// Counter is safe for concurrent use by multiple goroutines.
type Counter interface {
	Advance(now int64, delta int64) (count int64)
	// Revoke try to undo the delta of that historical moment.
	Revoke(hist int64, delta int64) (count int64)
	// Radvance will Revoke and then Advance.
	Radvance(now, hist int64, delta int64) (count int64)
	Zero()

	Duration() int64
	Dump() (start, end int64, deltas []int64, deltaStep int64)
}

type accumulator struct {
	start int64
	now   int64
	count int64
}

func NewAccumulator() Counter {
	return NewAccumulatorEx(0)
}

func NewAccumulatorEx(start int64) Counter {
	return &accumulator{start: start}
}

func (c *accumulator) Advance(now int64, delta int64) int64 {
	c.now = now
	return atomic.AddInt64(&c.count, delta)
}

func (c *accumulator) Revoke(hist int64, delta int64) int64 {
	return atomic.AddInt64(&c.count, -delta)
}

func (c *accumulator) Radvance(now, hist int64, delta int64) int64 {
	c.now = now
	return atomic.AddInt64(&c.count, 0)
}

func (c *accumulator) Zero() {
	atomic.StoreInt64(&c.count, 0)
}

func (c *accumulator) Duration() int64 {
	return c.now - c.start
}

func (c *accumulator) Dump() (start, end int64, deltas []int64, deltaStep int64) {
	start = c.start
	end = c.now
	deltas = []int64{c.count}
	deltaStep = end - start
	return
}

type slidingWindow struct {
	l     sync.Locker
	start int64
	step  int64
	slots []int64
	count int64
	now   int64
}

func NewSlidingWindow(start, window int64, slots int) Counter {
	return newSlidingWindow(start, window, slots, nil)
}

// NewSlidingWindowEx : use sync.Mutex when l is nil.
func NewSlidingWindowEx(start, window int64, slots int, l sync.Locker) Counter {
	return newSlidingWindow(start, window, slots, l)
}

func newSlidingWindow(start, window int64, slots int, l sync.Locker) *slidingWindow {
	if l == nil {
		l = &sync.Mutex{}
	}
	return &slidingWindow{
		l:     l,
		start: start,
		step:  window / int64(slots),
		slots: make([]int64, slots+1),
		count: 0,
		now:   start,
	}
}

func (c *slidingWindow) Zero() {
	c.l.Lock()
	defer c.l.Unlock()
	for i := 0; i < len(c.slots); i++ {
		c.slots[i] = 0
	}
	c.count = 0
}

func (c *slidingWindow) Advance(now int64, delta int64) int64 {
	c.l.Lock()
	defer c.l.Unlock()
	c.advance(now, delta)
	return c.calculate()
}

func (c *slidingWindow) Revoke(hist int64, delta int64) int64 {
	c.l.Lock()
	defer c.l.Unlock()
	c.revoke(hist, delta)
	return c.calculate()
}

func (c *slidingWindow) Radvance(now, hist int64, delta int64) int64 {
	c.l.Lock()
	defer c.l.Unlock()
	c.revoke(hist, delta)
	c.advance(now, delta)
	return c.calculate()
}

func (c *slidingWindow) Duration() int64 {
	c.l.Lock()
	defer c.l.Unlock()
	win := c.step * int64(len(c.slots)-1)
	dur := c.now - c.start
	if dur > win {
		dur = win
	}
	return dur
}

func (c *slidingWindow) advance(now int64, delta int64) {
	C := int64(len(c.slots))
	current := (c.now - c.start) / c.step
	if current < 0 {
		current = 0
	}
	next := (now - c.start) / c.step
	if next < current {
		next = current
	}

	// fast path
	if next == current {
		c.slots[next%C] += delta
		c.count += delta
		c.now = now
		return
	}

	// quick reset
	if next-current >= C {
		for i := int64(0); i < C; i++ {
			c.slots[i] = 0
		}
		c.slots[next%C] = delta
		c.count = delta
		c.now = now
		return
	}

	// other
	for i := current + 1; i <= next; i++ {
		c.count -= c.slots[i%C]
		c.slots[i%C] = 0
	}
	c.slots[next%C] += delta
	c.count += delta
	c.now = now
}

func (c *slidingWindow) revoke(hist int64, delta int64) {
	C := int64(len(c.slots))
	current := (c.now - c.start) / c.step
	if current < 0 {
		current = 0
	}
	prev := (hist - c.start) / c.step

	if prev >= 0 && current-prev >= 0 && current-prev < C {
		reduce := delta
		if reduce > c.slots[prev%C] {
			reduce = c.slots[prev%C]
		}
		c.slots[prev%C] -= reduce
		c.count -= reduce
	}
}

func (c *slidingWindow) calculate() int64 {
	C := int64(len(c.slots))
	current := (c.now - c.start) / c.step
	if current < 0 {
		return c.count
	}
	expired := c.slots[(current+1)%C]
	percent := float64((c.now-c.start)%c.step) / float64(c.step)
	return c.count - int64(float64(expired)*percent)
}

func (c *slidingWindow) Dump() (start, end int64, deltas []int64, deltaStep int64) {
	c.l.Lock()
	defer c.l.Unlock()

	C := int64(len(c.slots))
	current := (c.now - c.start) / c.step
	if current < 0 {
		current = 0
	}

	begin := int64(0)
	if current >= C {
		begin = current - (C - 1)
	}

	slots := make([]int64, 0, len(c.slots))
	for i := begin; i <= current; i++ {
		slots = append(slots, c.slots[i%C])
	}

	start = c.start + begin*c.step
	end = c.now
	deltas = slots
	deltaStep = c.step
	return
}

// LoadSlidingWindow : use sync.Mutex when l is nil.
func LoadSlidingWindow(start, window int64, slots int, l sync.Locker,
	end int64, deltas []int64, deltaStep int64) Counter {

	c := newSlidingWindow(start, window, slots, l)

	segs := int64(math.Max(math.Round(float64(deltaStep)/float64(c.step)), 1.0))

	for i := int64(0); i < int64(len(deltas)); i++ {
		delta := deltas[i]
		remain := delta
		now := start + i*deltaStep

		for j := int64(0); j < segs; j++ {
			if now >= end {
				now = end
				break
			}
			c.Advance(now, delta/segs)
			remain -= delta / segs
			now += deltaStep / segs
		}

		if now >= end {
			now = end
		}
		c.Advance(now, remain)
	}

	return c
}
