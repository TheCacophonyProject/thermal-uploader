// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"errors"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	ServerURL  string `yaml:"server-url"`
	Group      string `yaml:"group"`
	DeviceName string `yaml:"device-name"`
}

func (conf *Config) Validate() error {
	if conf.ServerURL == "" {
		return errors.New("server-url missing")
	}
	if conf.Group == "" {
		return errors.New("group missing")
	}
	if conf.DeviceName == "" {
		return errors.New("device-name missing")
	}
	return nil
}

type PrivateConfig struct {
	Password string `yaml:"password"`
}

func ParseConfigFile(filename string) (*Config, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseConfig(buf)
}

func ParseConfig(buf []byte) (*Config, error) {
	conf := new(Config)
	if err := yaml.Unmarshal(buf, conf); err != nil {
		return nil, err
	}
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

func ReadPassword(filename string) (string, error) {
	buf, err := ioutil.ReadFile(filename)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	var conf PrivateConfig
	if err := yaml.Unmarshal(buf, &conf); err != nil {
		return "", err
	}
	return conf.Password, nil
}

func WritePassword(filename, password string) error {
	conf := PrivateConfig{Password: password}
	buf, err := yaml.Marshal(&conf)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, buf, 0600)
}
