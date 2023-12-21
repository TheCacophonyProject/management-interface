/*
attiny-controller - Communicates with ATtiny microcontroller
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
	"errors"
	"log"
	"runtime"
	"strings"

	"github.com/godbus/dbus"
	"github.com/godbus/dbus/introspect"
)

const (
	dbusName = "org.cacophony.managementd"
	dbusPath = "/org/cacophony/managementd"
)

type service struct {
	nh *networkHandler
}

func startService(networkHandler *networkHandler) error {
	log.Println("Starting management service")
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	reply, err := conn.RequestName(dbusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return errors.New("name already taken")
	}

	s := &service{
		nh: networkHandler,
	}
	if err := conn.Export(s, dbusPath, dbusName); err != nil {
		return err
	}
	if err := conn.Export(genIntrospectable(s), dbusPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return err
	}
	log.Println("Started management service")
	return nil
}

func sendNewStateSignal(state string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		log.Fatalf("Failed to connect to Session Bus: %v", err)
	}

	err = conn.Emit(dbus.ObjectPath(dbusPath), dbusName+"."+"NewState", state)
	if err != nil {
		log.Fatalf("Failed to emit signal: %v", err)
	}
}

func sendNewNetworkState(state string) error {
	return sendBroadcast("NewNetworkState", []interface{}{state})
}

func sendBroadcast(signal string, payload []interface{}) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Emit(dbus.ObjectPath(dbusPath), dbusName+"."+signal, payload...)
}

func genIntrospectable(v interface{}) introspect.Introspectable {
	node := &introspect.Node{
		Interfaces: []introspect.Interface{{
			Name:    dbusName,
			Methods: introspect.Methods(v),
		}},
	}
	return introspect.NewIntrospectable(node)
}

func (s service) SetNetworkState(state string) *dbus.Error {
	if state == "wifi" {
		go runFuncLogErr(s.nh.setupWifiWithRollback)
	} else if state == "hotspot" {
		go runFuncLogErr(s.nh.setupHotspot)
	} else {
		return dbusErr(errors.New("invalid state"))
	}
	return nil
}

func (s service) GetNetworkState() (string, *dbus.Error) {
	return string(s.nh.state), nil
}

func runFuncLogErr(f func() error) {
	if err := f(); err != nil {
		log.Println("Error: ", err)
	}
}

func (s service) ReconfigureWifi() *dbus.Error {
	go runFuncLogErr(s.nh.reconfigureWifi)
	return nil
}

func dbusErr(err error) *dbus.Error {
	if err == nil {
		return nil
	}
	return &dbus.Error{
		Name: dbusName + "." + getCallerName(),
		Body: []interface{}{err.Error()},
	}
}

func getCallerName() string {
	fpcs := make([]uintptr, 1)
	n := runtime.Callers(3, fpcs)
	if n == 0 {
		return ""
	}
	caller := runtime.FuncForPC(fpcs[0] - 1)
	if caller == nil {
		return ""
	}
	funcNames := strings.Split(caller.Name(), ".")
	return funcNames[len(funcNames)-1]
}
