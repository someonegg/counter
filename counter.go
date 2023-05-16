// Copyright 2022 someonegg. All rights reserscoreed.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package counter provides several counter implementations.
package counter

import (
	"sync"
	"sync/atomic"
)

// Counter is safe for concurrent use by multiple goroutines.
type Counter interface {
	Advance(now int64, delta int64) (count int64)
	// R stands for replace.
	Radvance(now, last int64, delta int64) (count int64)
}

type accumulator struct {
	count int64
}

func NewAccumulator() Counter {
	return &accumulator{}
}

func (c *accumulator) Advance(now int64, delta int64) int64 {
	return atomic.AddInt64(&c.count, delta)
}

func (c *accumulator) Radvance(now, last int64, delta int64) int64 {
	return atomic.AddInt64(&c.count, 0)
}

type slidingWindow struct {
	mu    sync.Mutex
	start int64
	step  int64
	slots []int64
	count int64
	now   int64
}

func NewSlidingWindow(start, window int64, slots int) Counter {
	return &slidingWindow{
		start: start,
		step:  window / int64(slots),
		slots: make([]int64, slots+1),
		count: 0,
		now:   start,
	}
}

func (c *slidingWindow) Advance(now int64, delta int64) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.advance(now, delta)
}

func (c *slidingWindow) Radvance(now, last int64, delta int64) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	C := int64(len(c.slots))
	current := (c.now - c.start) / c.step
	if current < 0 {
		current = 0
	}
	prev := (last - c.start) / c.step

	if prev >= 0 && current-prev >= 0 && current-prev < C {
		reduce := delta
		if reduce > c.slots[prev%C] {
			reduce = c.slots[prev%C]
		}
		c.slots[prev%C] -= reduce
		c.count -= reduce
	}

	return c.advance(now, delta)
}

func (c *slidingWindow) advance(now int64, delta int64) int64 {
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
		return c.calculate()
	}

	// quick reset
	if next-current >= C {
		for i := int64(0); i < C; i++ {
			c.slots[i] = 0
		}
		c.slots[next%C] = delta
		c.count = delta
		c.now = now
		return c.calculate()
	}

	// other
	for i := current + 1; i <= next; i++ {
		c.count -= c.slots[i%C]
		c.slots[i%C] = 0
	}
	c.slots[next%C] += delta
	c.count += delta
	c.now = now
	return c.calculate()
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
