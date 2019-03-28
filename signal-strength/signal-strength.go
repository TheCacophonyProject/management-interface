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

package signalstrength

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"time"
)

const (
	modemURL  = "http://192.168.8.1"
	statusURL = modemURL + "/api/monitoring/status"
	timeout   = time.Second * 10
)

type responseHolder struct {
	response
}
type response struct {
	SignalIcon int
}

// Run returns signal strength of modem from 0 to 5
func Run() (int, error) {
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{
		Jar:     cookieJar,
		Timeout: timeout,
	}
	_, err = client.Get(modemURL) // Goto homepage first to get cookie for API request
	if err != nil {
		return 0, err
	}
	resp, err := client.Get(statusURL)
	if err != nil {
		return 0, err
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var r responseHolder
	err = xml.Unmarshal(bodyBytes, &r)
	if err != nil {
		return 0, err
	}
	return r.SignalIcon, nil
}
