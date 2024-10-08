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
	AdvanceEx(now int64, delta int64) (count, duration int64)

	// Revoke try to undo the delta of that historical moment.
	Revoke(hist int64, delta int64) (count int64)
	RevokeEx(hist int64, delta int64) (count, duration int64)

	// Radvance will Revoke and then Advance.
	Radvance(now, hist int64, delta int64) (count int64)
	RadvanceEx(now, hist int64, delta int64) (count, duration int64)

	// Time will not advance.
	Count() int64
	Duration() int64

	Dump() (start, end int64, deltas []int64, deltaStep int64)

	Zero()
}

type accumulator struct {
	start int64
	now   int64
	count int64
}

func NewAccumulator(start int64) Counter {
	return &accumulator{start: start}
}

func (c *accumulator) Zero() {
	atomic.StoreInt64(&c.count, 0)
}

func (c *accumulator) Advance(now int64, delta int64) int64 {
	c.now = now
	return atomic.AddInt64(&c.count, delta)
}

func (c *accumulator) AdvanceEx(now int64, delta int64) (int64, int64) {
	return c.Advance(now, delta), c.Duration()
}

func (c *accumulator) Revoke(hist int64, delta int64) int64 {
	return atomic.AddInt64(&c.count, -delta)
}

func (c *accumulator) RevokeEx(hist int64, delta int64) (int64, int64) {
	return c.Revoke(hist, delta), c.Duration()
}

func (c *accumulator) Radvance(now, hist int64, delta int64) int64 {
	c.now = now
	return c.count
}

func (c *accumulator) RadvanceEx(now, hist int64, delta int64) (int64, int64) {
	return c.Radvance(now, hist, delta), c.Duration()
}

func (c *accumulator) Count() int64 {
	return c.count
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

type slidingWindow[L any, PL locker[L]] struct {
	l     L
	start int64
	step  int64
	slots []int64
	count int64
	now   int64
}

func NewSlidingWindow(start, window int64, slots int) Counter {
	return newSlidingWindow[sync.Mutex](start, window, slots)
}

func NewSlidingWindowNoLock(start, window int64, slots int) Counter {
	return newSlidingWindow[nopLocker](start, window, slots)
}

func newSlidingWindow[L any, PL locker[L]](start, window int64, slots int) *slidingWindow[L, PL] {
	return &slidingWindow[L, PL]{
		start: start,
		step:  window / int64(slots),
		slots: make([]int64, slots+1),
		count: 0,
		now:   start,
	}
}

func (c *slidingWindow[L, PL]) Zero() {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	for i := 0; i < len(c.slots); i++ {
		c.slots[i] = 0
	}
	c.count = 0
}

func (c *slidingWindow[L, PL]) Advance(now int64, delta int64) int64 {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	c.advance(now, delta)
	return c.calculate()
}

func (c *slidingWindow[L, PL]) AdvanceEx(now int64, delta int64) (int64, int64) {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	c.advance(now, delta)
	return c.calculate(), c.duration()
}

func (c *slidingWindow[L, PL]) Revoke(hist int64, delta int64) int64 {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	c.revoke(hist, delta)
	return c.calculate()
}

func (c *slidingWindow[L, PL]) RevokeEx(hist int64, delta int64) (int64, int64) {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	c.revoke(hist, delta)
	return c.calculate(), c.duration()
}

func (c *slidingWindow[L, PL]) Radvance(now, hist int64, delta int64) int64 {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	c.revoke(hist, delta)
	c.advance(now, delta)
	return c.calculate()
}

func (c *slidingWindow[L, PL]) RadvanceEx(now, hist int64, delta int64) (int64, int64) {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	c.revoke(hist, delta)
	c.advance(now, delta)
	return c.calculate(), c.duration()
}

func (c *slidingWindow[L, PL]) Count() int64 {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	return c.calculate()
}

func (c *slidingWindow[L, PL]) Duration() int64 {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()
	return c.duration()
}

func (c *slidingWindow[L, PL]) advance(now int64, delta int64) {
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

func (c *slidingWindow[L, PL]) revoke(hist int64, delta int64) {
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

func (c *slidingWindow[L, PL]) calculate() int64 {
	C := int64(len(c.slots))
	current := (c.now - c.start) / c.step
	if current < 0 {
		return c.count
	}
	expired := c.slots[(current+1)%C]
	percent := float64((c.now-c.start)%c.step) / float64(c.step)
	return c.count - int64(float64(expired)*percent)
}

func (c *slidingWindow[L, PL]) duration() int64 {
	win := c.step * int64(len(c.slots)-1)
	dur := c.now - c.start
	if dur > win {
		dur = win
	}
	return dur
}

func (c *slidingWindow[L, PL]) Dump() (start, end int64, deltas []int64, deltaStep int64) {
	PL(&c.l).Lock()
	defer PL(&c.l).Unlock()

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

func LoadSlidingWindow(
	start, end int64, deltas []int64, deltaStep int64,
	window int64, slots int) Counter {

	c := newSlidingWindow[sync.Mutex](start, window, slots)
	c.load(start, end, deltas, deltaStep)
	return c
}

func LoadSlidingWindowNoLock(
	start, end int64, deltas []int64, deltaStep int64,
	window int64, slots int) Counter {

	c := newSlidingWindow[nopLocker](start, window, slots)
	c.load(start, end, deltas, deltaStep)
	return c
}

func (c *slidingWindow[L, PL]) load(start, end int64, deltas []int64, deltaStep int64) {
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
}
