// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rjeczalik/notify"
)

const configFilename = "uploader.yaml"
const privConfigFilename = "uploader-priv.yaml"
const cptvGlob = "*.cptv"

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
		log.Println("first time registration - saving password")
		err := WritePassword(privConfigFilename, api.Password())
		if err != nil {
			return err
		}
	}

	log.Println("watching", conf.Directory)
	fsEvents := make(chan notify.EventInfo, 1)
	if err := notify.Watch(conf.Directory, fsEvents, notify.InCloseWrite, notify.InMovedTo); err != nil {
		return err
	}
	defer notify.Stop(fsEvents)
	for {
		// Check for files to upload first in case there are CPTV
		// files around when the uploader starts.
		if err := uploadFiles(api, conf.Directory); err != nil {
			return err
		}
		// Block until there's activity in the directory. We don't
		// care what it is as uploadFiles will only act on CPTV
		// files.
		<-fsEvents
	}
	return nil
}

func uploadFiles(api *CacophonyAPI, directory string) error {
	matches, _ := filepath.Glob(filepath.Join(directory, cptvGlob))
	for _, filename := range matches {
		err := uploadFile(api, filename)
		if err != nil {
			return err
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
