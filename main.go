// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rjeczalik/notify"
)

const configFilename = "uploader.yaml"
const privConfigFilename = "uploader-priv.yaml"
const cptvExtension = ".cptv"

func main() {
	err := runMain()
	if err != nil {
		log.Fatal(err.Error())
	}
}

func runMain() error {
	conf, err := ParseConfigFile(configFilename)
	if err != nil {
		return fmt.Errorf("configuration error: %v", err)
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

	// XXX handle pre-existing files on startup

	log.Println("watching", conf.Directory)

	// Use a big channel buffer so that events are kept even if
	// uploads take a while.
	fsEvents := make(chan notify.EventInfo, 64)
	if err := notify.Watch(conf.Directory, fsEvents, notify.InCloseWrite, notify.InMovedTo); err != nil {
		return err
	}
	defer notify.Stop(fsEvents)
	for {
		event := <-fsEvents
		if strings.HasSuffix(event.Path(), cptvExtension) {
			err := uploadFile(api, event.Path())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func uploadFile(api *CacophonyAPI, filename string) error {
	log.Printf("uploading: %s", filename)
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		// File disappeared since the event was generated. Ignore.
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()
	if err := api.UploadThermalRaw(f); err != nil {
		return err
	}
	log.Printf("upload complete: %s", filename)
	return os.Remove(filename)
}
