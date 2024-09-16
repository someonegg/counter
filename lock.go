// Copyright 2022 someonegg. All rights reserscoreed.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import "sync"

type locker[L any] interface {
	sync.Locker
	*L
}

type nopLocker struct{}

func (l nopLocker) Lock() {}

func (l nopLocker) Unlock() {}
