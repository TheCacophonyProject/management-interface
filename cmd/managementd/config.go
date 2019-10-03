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
	"fmt"

	goconfig "github.com/TheCacophonyProject/go-config"
)

// Config for management interface
type Config struct {
	Port    int
	CPTVDir string
	config  *goconfig.Config
}

func (c Config) String() string {
	return fmt.Sprintf("{ Port: %d, CPTVDir: %s }", c.Port, c.CPTVDir)
}

// ParseConfig parses the config
func ParseConfig(configDir string) (*Config, error) {
	config, err := goconfig.New(configDir)
	if err != nil {
		return nil, err
	}

	ports := goconfig.DefaultPorts()
	if err := config.Unmarshal(goconfig.PortsKey, &ports); err != nil {
		return nil, err
	}

	thermalRecorder := goconfig.DefaultThermalRecorder()
	if err := config.Unmarshal(goconfig.ThermalRecorderKey, &thermalRecorder); err != nil {
		return nil, err
	}

	return &Config{
		Port:    ports.Managementd,
		CPTVDir: thermalRecorder.OutputDir,
		config:  config,
	}, nil
}
