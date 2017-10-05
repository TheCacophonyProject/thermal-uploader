// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"log"
)

const configFilename = "uploader.yaml"
const privConfigFilename = "uploader-priv.yaml"

func main() {
	err := runMain()
	if err != nil {
		log.Fatal(err.Error())
	}
}

func runMain() error {
	conf, err := ParseConfigFile(configFilename)
	if err != nil {
		return err
	}
	password, err := ReadPassword(privConfigFilename)
	if err != nil {
		return err
	}
	api, err := NewAPI(conf.ServerURL, conf.Group, conf.DeviceName, password)
	if err != nil {
		return err
	}
	if api.JustRegistered() {
		log.Println("First time, registration, saving password")
		err := WritePassword(privConfigFilename, api.Password())
		if err != nil {
			return err
		}
	}
	return nil
}
