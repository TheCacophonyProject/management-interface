// go-api - Client for the Cacophony API server.
// Copyright (C) 2018, The Cacophony Project
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package api

import (
	"math/rand"
	"time"
)

const (
	chars       = "abcdefghijklmnopqrstuvwxyz0123456789"
	charIdxBits = 6                  // 6 bits to represent a char index
	charIdxMax  = 63 / charIdxBits   // # of char indices fitting in 63 bits
	charIdxMask = 1<<charIdxBits - 1 // All 1-bits, as many as charIdxBits
)

var randSrc = rand.NewSource(time.Now().UnixNano())

func randString(n int) string {
	b := make([]byte, n)
	// A randSrc.Int63() generates 63 random bits, enough for charIdxMax characters!
	for i, cache, remain := n-1, randSrc.Int63(), charIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), charIdxMax
		}
		if idx := int(cache & charIdxMask); idx < len(chars) {
			b[i] = chars[idx]
			i--
		}
		cache >>= charIdxBits
		remain--
	}
	return string(b)
}
