// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	cptv "github.com/TheCacophonyProject/go-cptv"
	arg "github.com/alexflint/go-arg"
	"github.com/rjeczalik/notify"
)

const cptvGlob = "*.cptv"

type Args struct {
	ConfigFile string `arg:"-c,--config" help:"path to configuration file"`
}

func procArgs() Args {
	var args Args
	args.ConfigFile = "/etc/thermal-uploader.yaml"
	arg.MustParse(&args)
	return args
}

func main() {
	err := runMain()
	if err != nil {
		log.Fatal(err.Error())
	}
}

func runMain() error {
	log.SetFlags(0) // Removes default timestamp flag

	args := procArgs()
	conf, err := ParseConfigFile(args.ConfigFile)
	if err != nil {
		return fmt.Errorf("configuration error: %v", err)
	}
	privConfigFilename := genPrivConfigFilename(args.ConfigFile)
	log.Println("private settings file:", privConfigFilename)
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

func genPrivConfigFilename(confFilename string) string {
	dirname, filename := filepath.Split(confFilename)
	bareFilename := strings.TrimSuffix(filename, ".yaml")
	return filepath.Join(dirname, bareFilename+"-priv.yaml")
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

	info, err := extractCPTVInfo(filename)
	if err != nil {
		log.Println("failed to extract CPTV info from file. Deleting CPTV file")
		return os.Remove(filename)
	}
	log.Printf("ts=%s duration=%ds", info.timestamp, info.duration)

	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		// File disappeared since the event was generated. Ignore.
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	if err := api.UploadThermalRaw(info, br); err != nil {
		return err
	}
	log.Printf("upload complete: %s", filename)
	return os.Remove(filename)
}

type cptvInfo struct {
	timestamp time.Time
	duration  int
}

func extractCPTVInfo(filename string) (*cptvInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// TODO: use the higher level cptv.Reader type (when it exists!)
	p, err := cptv.NewParser(bufio.NewReader(file))
	if err != nil {
		return nil, err
	}
	fields, err := p.Header()
	if err != nil {
		return nil, err
	}
	timestamp, err := fields.Timestamp(cptv.Timestamp)
	if err != nil {
		return nil, err
	}

	frames := 0
	for {
		_, _, err := p.Frame()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		frames++
	}
	return &cptvInfo{
		timestamp: timestamp,
		duration:  frames / 9,
	}, nil
}
