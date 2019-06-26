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
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gofrs/flock"
	"github.com/spf13/afero"
	yaml "gopkg.in/yaml.v2"
)

const (
	DeviceConfigPath     = "/etc/cacophony/device.yaml"
	RegisteredConfigPath = "/etc/cacophony/device-priv.yaml"
)

type Config struct {
	ServerURL  string `yaml:"server-url" json:"serverURL"`
	Group      string `yaml:"group" json:"groupname"`
	DeviceName string `yaml:"device-name" json:"devicename"`
}

type PrivateConfig struct {
	Password string `yaml:"password"`
	DeviceID int    `yaml:"device-id" json:"deviceID"`
}

//Validate checks supplied Config contains the required data
func (conf *PrivateConfig) IsValid() bool {
	if conf.Password == "" {
		return false
	}

	if conf.DeviceID == 0 {
		return false
	}
	return true
}

//Validate checks supplied Config contains the required data
func (conf *Config) Validate() error {
	if conf.ServerURL == "" {
		return errors.New("server-url missing")
	}

	if conf.DeviceName == "" {
		return errors.New("device-name missing")
	}
	return nil
}

// LoadConfig from deviceConfigPath with a read lock
func LoadConfig() (*Config, error) {
	buf, err := afero.ReadFile(Fs, DeviceConfigPath)
	if err != nil {
		return nil, err
	}
	return ParseConfig(buf)
}

//ParseConfig takes supplied bytes and returns a parsed Config struct
func ParseConfig(buf []byte) (*Config, error) {
	conf := &Config{}

	if err := yaml.Unmarshal(buf, conf); err != nil {
		return nil, err
	}
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

const (
	lockfile       = "/var/lock/go-api-priv.lock"
	lockRetryDelay = 678 * time.Millisecond
	lockTimeout    = 5 * time.Second
)

// LoadPrivateConfig acquires a readlock and reads private config
func LoadPrivateConfig() (*PrivateConfig, error) {
	lockSafeConfig := NewLockSafeConfig(RegisteredConfigPath)
	return lockSafeConfig.Read()
}

type LockSafeConfig struct {
	fileLock *flock.Flock
	filename string
	config   *PrivateConfig
}

func NewLockSafeConfig(filename string) *LockSafeConfig {
	return &LockSafeConfig{
		filename: filename,
		fileLock: flock.New(lockfile),
	}
}

func (lockSafeConfig *LockSafeConfig) Unlock() {
	lockSafeConfig.fileLock.Unlock()
}

// GetExLock acquires an exclusive lock on confPassword
func (lockSafeConfig *LockSafeConfig) GetExLock() (bool, error) {
	lockCtx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	locked, err := lockSafeConfig.fileLock.TryLockContext(lockCtx, lockRetryDelay)
	return locked, err
}

// getReadLock  acquires a read lock on the supplied Flock struct
func getReadLock(fileLock *flock.Flock) (bool, error) {
	lockCtx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	locked, err := fileLock.TryRLockContext(lockCtx, lockRetryDelay)
	return locked, err
}

// ReadPassword acquires a readlock and reads the config
func (lockSafeConfig *LockSafeConfig) Read() (*PrivateConfig, error) {
	locked := lockSafeConfig.fileLock.Locked()
	if locked == false {
		locked, err := getReadLock(lockSafeConfig.fileLock)
		if locked == false || err != nil {
			return nil, err
		}
		defer lockSafeConfig.Unlock()
	}

	buf, err := afero.ReadFile(Fs, lockSafeConfig.filename)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(buf, &lockSafeConfig.config); err != nil {
		return nil, err
	}
	return lockSafeConfig.config, nil
}

// WritePassword checks the file is locked and writes the password
func (lockSafeConfig *LockSafeConfig) Write(deviceID int, password string) error {
	conf := PrivateConfig{DeviceID: deviceID, Password: password}
	buf, err := yaml.Marshal(&conf)
	if err != nil {
		return err
	}
	if lockSafeConfig.fileLock.Locked() {
		err = afero.WriteFile(Fs, lockSafeConfig.filename, buf, 0600)
	} else {
		return fmt.Errorf("WritePassword could not get file lock %v", lockSafeConfig.filename)
	}
	return err
}

var Fs = afero.NewOsFs()
