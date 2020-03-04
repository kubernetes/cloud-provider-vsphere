/*
 Copyright 2020 The Kubernetes Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package loadbalancer

import (
	"fmt"
	"sync"
)

type keyLock struct {
	lock sync.Mutex
	keys map[string]*sync.Mutex
}

func newKeyLock() *keyLock {
	return &keyLock{keys: map[string]*sync.Mutex{}}
}

// Lock locks the key
func (l *keyLock) Lock(key string) {
	l.lock.Lock()
	lock := l.keys[key]
	if lock == nil {
		lock = &sync.Mutex{}
		l.keys[key] = lock
	}
	l.lock.Unlock()

	lock.Lock()
}

// Unlock unlocks the key
func (l *keyLock) Unlock(key string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	lock := l.keys[key]
	if lock == nil {
		panic(fmt.Sprintf("unlock of unknown keyLock %s", key))
	}
	lock.Unlock()
}
