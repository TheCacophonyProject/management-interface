/*
management-interface - Web based management of Raspberry Pis over WiFi
Copyright (C) 2018, The Cacophony Project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package managementinterface

func newClientError(msg string) *clientError {
	return &clientError{msg}
}

// clientError represents an error that is caused by the HTTP client
// at the other end of a request.
type clientError struct {
	msg string
}

func (e *clientError) Error() string {
	return e.msg
}

func isClientError(err error) bool {
	_, ok := err.(*clientError)
	return ok
}
