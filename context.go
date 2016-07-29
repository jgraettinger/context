// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"net/http"
	"sync"
	"time"
	"unsafe"
)

// Mutexes and maps are partitioned by *Request to reduce contention on
// the respective maps and locks.
const numberOfBags = 128

type databag struct {
	mutex sync.RWMutex
	data  map[*http.Request]map[interface{}]interface{}
	datat map[*http.Request]int64
}

var bags [numberOfBags]databag

func init() {
	for i := range bags {
		bags[i].data = make(map[*http.Request]map[interface{}]interface{})
		bags[i].datat = make(map[*http.Request]int64)
	}
}

// Maps a *Request to a databag to use.
func requestBag(r *http.Request) *databag {
	var offset = uintptr(unsafe.Pointer(r)) % numberOfBags
	return &bags[offset]
}

// Set stores a value for a given key in a given request.
func Set(r *http.Request, key, val interface{}) {
	var b = requestBag(r)
	b.mutex.Lock()
	if b.data[r] == nil {
		b.data[r] = make(map[interface{}]interface{})
		b.datat[r] = time.Now().Unix()
	}
	b.data[r][key] = val
	b.mutex.Unlock()
}

// Get returns a value stored for a given key in a given request.
func Get(r *http.Request, key interface{}) interface{} {
	var b = requestBag(r)
	b.mutex.RLock()
	if ctx := b.data[r]; ctx != nil {
		value := ctx[key]
		b.mutex.RUnlock()
		return value
	}
	b.mutex.RUnlock()
	return nil
}

// GetOk returns stored value and presence state like multi-value return of map access.
func GetOk(r *http.Request, key interface{}) (interface{}, bool) {
	var b = requestBag(r)
	b.mutex.RLock()
	if _, ok := b.data[r]; ok {
		value, ok := b.data[r][key]
		b.mutex.RUnlock()
		return value, ok
	}
	b.mutex.RUnlock()
	return nil, false
}

// GetAll returns all stored values for the request as a map. Nil is returned for invalid requests.
func GetAll(r *http.Request) map[interface{}]interface{} {
	var b = requestBag(r)
	b.mutex.RLock()
	if context, ok := b.data[r]; ok {
		result := make(map[interface{}]interface{}, len(context))
		for k, v := range context {
			result[k] = v
		}
		b.mutex.RUnlock()
		return result
	}
	b.mutex.RUnlock()
	return nil
}

// GetAllOk returns all stored values for the request as a map and a boolean value that indicates if
// the request was registered.
func GetAllOk(r *http.Request) (map[interface{}]interface{}, bool) {
	var b = requestBag(r)
	b.mutex.RLock()
	context, ok := b.data[r]
	result := make(map[interface{}]interface{}, len(context))
	for k, v := range context {
		result[k] = v
	}
	b.mutex.RUnlock()
	return result, ok
}

// Delete removes a value stored for a given key in a given request.
func Delete(r *http.Request, key interface{}) {
	var b = requestBag(r)
	b.mutex.Lock()
	if b.data[r] != nil {
		delete(b.data[r], key)
	}
	b.mutex.Unlock()
}

// Clear removes all values stored for a given request.
//
// This is usually called by a handler wrapper to clean up request
// variables at the end of a request lifetime. See ClearHandler().
func Clear(r *http.Request) {
	var b = requestBag(r)
	b.mutex.Lock()
	clear(r)
	b.mutex.Unlock()
}

// clear is Clear without the lock.
func clear(r *http.Request) {
	var b = requestBag(r)
	delete(b.data, r)
	delete(b.datat, r)
}

/*
// Purge removes request data stored for longer than maxAge, in seconds.
// It returns the amount of requests removed.
//
// If maxAge <= 0, all request data is removed.
//
// This is only used for sanity check: in case context cleaning was not
// properly set some request data can be kept forever, consuming an increasing
// amount of memory. In case this is detected, Purge() must be called
// periodically until the problem is fixed.
func Purge(maxAge int) int {
	mutex.Lock()
	count := 0
	if maxAge <= 0 {
		count = len(data)
		data = make(map[*http.Request]map[interface{}]interface{})
		datat = make(map[*http.Request]int64)
	} else {
		min := time.Now().Unix() - int64(maxAge)
		for r := range data {
			if datat[r] < min {
				clear(r)
				count++
			}
		}
	}
	mutex.Unlock()
	return count
}
*/

// ClearHandler wraps an http.Handler and clears request values at the end
// of a request lifetime.
func ClearHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer Clear(r)
		h.ServeHTTP(w, r)
	})
}
