// Copyright 2022 someonegg. All rights reserscoreed.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import (
	"testing"
	"time"
)

func TestSlidingWindow(t *testing.T) {
	now := time.Now().UnixMilli()

	c := NewSlidingWindow(now, int64(time.Minute), 60)

	now += int64(time.Second) / 5

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += int64(time.Second)
	}
	count := c.Advance(now, 0)
	t.Log(count, c)
	if count != 598 {
		t.FailNow()
	}

	now += 6 * int64(time.Second) / 5
	count = c.Advance(now, 0)
	t.Log(count, c)
	if count != 586 {
		t.FailNow()
	}

	now += int64(2 * time.Minute)
	count = c.Advance(now, 20)
	t.Log(count, c)
	if count != 20 {
		t.FailNow()
	}

	for i := 0; i < 60; i++ {
		c.Advance(now, 10)
		now += int64(time.Second)
	}
	count = c.Advance(now, 0)
	t.Log(count, c)
	if count != 608 {
		t.FailNow()
	}
}
