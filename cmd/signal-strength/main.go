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

package main

import (
	"github.com/TheCacophonyProject/go-utils/logging"
	signalstrength "github.com/TheCacophonyProject/management-interface/signal-strength"
)

var log = logging.NewLogger("info")

func main() {
	sig, err := signalstrength.Run()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(sig)
}
