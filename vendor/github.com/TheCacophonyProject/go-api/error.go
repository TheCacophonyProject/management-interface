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

// Error is returned by API calling methods. As well as an error
// message, it includes whether the error is permanent or not.
type Error struct {
	message   string
	permanent bool
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.message
}

// Permanent returns true if the error is permanent. Operations
// resulting in non-permanent/temporary errors may be retried.
func (e *Error) Permanent() bool {
	return e.permanent
}

// IsPermanentError examines the supplied error and returns true if it
// is permanent.
func IsPermanentError(err error) bool {
	if err == nil {
		return false
	}
	if apiErr, ok := err.(*Error); ok {
		return apiErr.Permanent()
	}
	// non-Errors are considered permanent.
	return true
}

func temporaryError(err error) *Error {
	return &Error{message: err.Error(), permanent: false}
}
