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

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += second
	}
	count := c.Advance(now, 0)
	t.Log(count, c)
	if count != 598 {
		t.FailNow()
	}

	now += 6 * second / 5
	count = c.Radvance(now, start, 0)
	t.Log(count, c)
	if count != 586 {
		t.FailNow()
	}

	now += int64(2 * time.Minute)
	count = c.Radvance(now, 0, 20)
	t.Log(count, c)
	if count != 20 {
		t.FailNow()
	}

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		c.Radvance(now, now-second, 2)
		now += second
	}
	count = c.Advance(now, 0)
	t.Log(count, c)
	if count != 610 {
		t.FailNow()
	}
	count = c.Radvance(now, now-second, 15)
	t.Log(count, c)
	if count != 613 {
		t.FailNow()
	}
}
