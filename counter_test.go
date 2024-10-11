// Copyright 2022 someonegg. All rights reserscoreed.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import (
	"testing"
	"time"
)

func TestSlidingWindow(t *testing.T) {
	minute := int64(time.Minute / time.Millisecond)
	second := int64(time.Second / time.Millisecond)

	start := time.Now().UnixMilli()
	now := start

	c := NewSlidingWindow(now, minute, 60)

	now += second / 5
	count := c.Advance(now, 0)
	dur := c.Duration()
	t.Log(count, c, dur)

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += second
	}
	count = c.Advance(now, 0)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 598 || dur != c.Duration() {
		t.FailNow()
	}

	c = NewSlidingWindowNoLock(now, minute, 60)

	now += second / 5
	count = c.Advance(now, 0)
	dur = c.Duration()
	t.Log(count, c, dur)

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += second
	}
	count = c.Advance(now, 0)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 598 || dur != c.Duration() {
		t.FailNow()
	}

	now += 6 * second / 5
	count = c.Radvance(now, start, 0)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 586 || dur != c.Duration() {
		t.FailNow()
	}

	now += int64(2 * time.Minute)
	count = c.Radvance(now, 0, 20)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 20 {
		t.FailNow()
	}

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		if i%2 == 0 {
			c.Revoke(now-second, 2)
			c.Advance(now, 2)
		} else {
			c.Radvance(now, now-second, 2)
		}
		now += second
	}
	count = c.Advance(now, 0)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 610 || dur != c.Duration() {
		t.FailNow()
	}
	count = c.Radvance(now, now-second, 15)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 613 || dur != c.Duration() {
		t.FailNow()
	}

	c.Zero()

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += second
	}
	count = c.Advance(now, 0)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 596 || dur != c.Duration() {
		t.FailNow()
	}

	start, end, step, deltas := c.(Dumper).Dump()
	t.Log(start, end, step, deltas)

	c2 := NewSlidingWindow(now, minute, 180)
	c2.(Loader).Load(start, end, step, deltas)
	count = c2.Advance(now, 0)
	dur = c2.Duration()
	t.Log(count, c2, dur)
	if count != 596 {
		t.FailNow()
	}

	c3 := NewSlidingWindowNoLock(now, minute, 180)
	c3.(Loader).Load(start, end, step, deltas)
	count = c3.Advance(now, 0)
	dur = c3.Duration()
	t.Log(count, c3, dur)
	if count != 596 {
		t.FailNow()
	}
}
