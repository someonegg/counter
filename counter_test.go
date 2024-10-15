// Copyright 2022 someonegg. All rights reserscoreed.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import (
	"testing"
	"time"
)

var (
	minute = int64(time.Minute / time.Millisecond)
	second = int64(time.Second / time.Millisecond)
)

func TestSlidingWindow(t *testing.T) {
	var count, dur int64
	query := func(c Counter, now int64) {
		count = c.Advance(now, 0)
		dur = c.Duration()
		t.Log(count, dur, c)
	}

	now := time.Now().UnixMilli()
	c := NewSlidingWindow(now, minute, 60)

	now += second / 5
	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += second
	}

	query(c, now)
	if count != 598 {
		t.FailNow()
	}

	c = NewSlidingWindowNoLock(now, minute, 60)

	now += second / 5
	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += second
	}

	query(c, now)
	if count != 598 {
		t.FailNow()
	}

	now += 6 * second / 5

	query(c, now)
	if count != 586 {
		t.FailNow()
	}

	now += int64(2 * time.Minute)
	c.Radvance(now, 0, 20)

	query(c, now)
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

	query(c, now)
	if count != 610 {
		t.FailNow()
	}

	count = c.Radvance(now, now-second, 15)
	dur = c.Duration()
	t.Log(count, c, dur)
	if count != 613 {
		t.FailNow()
	}

	c.Zero()

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += second
	}

	query(c, now)
	if count != 596 {
		t.FailNow()
	}

	start, end, step, deltas := c.(Dumper).Dump()
	t.Log(start, end, step, deltas)

	c2 := NewSlidingWindow(now, minute, 180)
	c2.(Loader).Load(start, end, step, deltas)
	query(c2, now)
	if count != 596 {
		t.FailNow()
	}

	c3 := NewSlidingWindowNoLock(now, minute, 180)
	c3.(Loader).Load(start, end, step, deltas)
	query(c3, now)
	if count != 596 {
		t.FailNow()
	}
}
