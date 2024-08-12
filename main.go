// thermal-uploader - upload thermal video recordings in CPTV format to the project's API server.
//  Copyright (C) 2017, The Cacophony Project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/TheCacophonyProject/go-api"
	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/modemd/connrequester"
	arg "github.com/alexflint/go-arg"
	"github.com/godbus/dbus"
	"github.com/rjeczalik/notify"
)

const (
	failedUploadsDir        = "failed-uploads"
	connectionTimeout       = time.Minute * 2
	connectionRetryInterval = time.Minute * 10
	failedRetryInterval     = time.Minute * 10
	failedRetryMaxInterval  = time.Hour * 24
)

var version = "No version provided"
var globs = [5]string{"*.cptv", "*.avi", "*.mp4", "*.wav", "*.aac"}

type Args struct {
	ConfigDir string `arg:"-c,--config" help:"path to configuration directory"`
}

func (Args) Version() string {
	return version
}

func procArgs() Args {
	var args Args
	args.ConfigDir = goconfig.DefaultConfigDir
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
	log.Printf("running version: %s", version)

	cr := connrequester.NewConnectionRequester()
	log.Println("requesting internet connection")
	cr.Start()
	cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
	log.Println("internet connection made")

	apiClient, err := api.New()
	if api.IsNotRegisteredError(err) {
		log.Println("device not registered. Exiting and waiting to be restarted")
		os.Exit(0)
	} else if err != nil {
		return err
	}
	cr.Stop()

	conf, err := ParseConfig(args.ConfigDir)
	if err != nil {
		return fmt.Errorf("configuration error: %v", err)
	}

	log.Println("making failed uploads directory")
	os.MkdirAll(filepath.Join(conf.Directory, failedUploadsDir), 0755)

	log.Println("watching", conf.Directory)
	fsEvents := make(chan notify.EventInfo, 1)
	if err := notify.Watch(conf.Directory, fsEvents, notify.InCloseWrite, notify.InMovedTo); err != nil {
		return err
	}

	nextFailedRetry := time.Now()
	failedRetryAttempts := 0
	defer notify.Stop(fsEvents)
	sendOnRequest(20)
	for {
		newFiles := 0
		// Check for files to upload first in case there are CPTV
		// files around when the uploader starts.
		cr.Start()
		cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
		if newFiles, err = uploadFiles(apiClient, conf.Directory); err != nil {
			return err
		}

		//try failed uploads again if succeeded
		if time.Now().After(nextFailedRetry) {
			if retryFailedUploads(apiClient, conf.Directory) {
				failedRetryAttempts = 0
				nextFailedRetry = time.Now()
			} else {
				failedRetryAttempts += 1
				timeAddition := failedRetryInterval * time.Duration(failedRetryAttempts*failedRetryAttempts)
				nextFailedRetry = time.Now().Add(minDuration(timeAddition, failedRetryMaxInterval))
				log.Printf("Failed still failed try again after %v", nextFailedRetry)
			}
		}
		if newFiles == 0 {
			sendFinished()
		}
		cr.Stop()
		// Block until there's activity in the directory. We don't
		// care what it is as uploadFiles will only act on CPTV
		// files.
		<-fsEvents
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a > b {
		return b
	} else {
		return a
	}
}

func uploadFiles(apiClient *api.CacophonyAPI, directory string) (int, error) {
	var matches = make([]string, 0, 5)
	for _, glob := range globs {
		globMatches, _ := filepath.Glob(filepath.Join(directory, glob))
		matches = append(matches, globMatches...)
	}

	var err error
	for _, filename := range matches {
		if err != nil {
			log.Printf("Failed to send on request %v", err)
		}

		job := newUploadJob(filename)
		err = job.preprocess()
		if err != nil {
			log.Printf("Failed to preprocess %v: %v", filename, err)
			job.moveToFailed()
			continue
		}
		err = uploadFileWithRetries(apiClient, job)
		if err != nil {
			return len(matches), err
		}
	}
	return len(matches), nil
}

func retryFailedUploads(apiClient *api.CacophonyAPI, directory string) bool {
	var matches = make([]string, 0, 5)
	for _, glob := range globs {
		globMatches, _ := filepath.Glob(filepath.Join(directory, failedUploadsDir, glob))
		matches = append(matches, globMatches...)
	}
	if len(matches) == 0 {
		return true
	}
	// start at a random index to avoid always failing on the same file
	startIndex := rand.Intn(len(matches))
	for i := 0; i < len(matches); i++ {
		index := (startIndex + i) % len(matches)
		filename := matches[index]
		job := newUploadJob(filename)
		err := job.upload(apiClient)

		if err != nil {
			log.Printf("Uploading still failing to upload %v: %v", filename, err)
			return false
		}
		log.Print("success uploading failed items")
	}
	return true
}

func uploadFileWithRetries(apiClient *api.CacophonyAPI, job *uploadJob) error {
	log.Printf("uploading: %s", job.filename)
	for remainingTries := 2; remainingTries >= 0; remainingTries-- {
		err := job.upload(apiClient)
		if err == nil {
			log.Printf("upload complete %v", job.filename)
			return nil
		}
		log.Printf("upload failed: %v", err)
		if remainingTries > 0 {
			log.Printf("trying %d more times", remainingTries)
		}
	}
	log.Printf("upload failed multiple times, moving file to failed uploads folder")
	return job.moveToFailed()
}

const dbusDest = "org.cacophony.ATtiny"
const dbusPath = "/org/cacophony/ATtiny"

func getDbusObj() (dbus.BusObject, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	obj := conn.Object(dbusDest, dbusPath)
	return obj, nil
}

func sendFinished() error {
	attempt := 0
	obj, err := getDbusObj()
	if err != nil {
		return err
	}
	for attempt < 3 {
		err = obj.Call("org.cacophony.ATtiny.StayOnFinished", 0, "uploader").Store()
		if err == nil {
			return nil
		}
		attempt += 1
		if attempt < 3 {
			log.Printf("Retrying finished request %v", err)
			time.Sleep(1 * time.Second)
		}
	}
	return err
}

func sendOnRequest(timeOn int64) error {
	attempt := 0
	obj, err := getDbusObj()
	if err != nil {
		return err
	}
	for attempt < 3 {

		err = obj.Call("org.cacophony.ATtiny.StayOnForProcess", 0, "uploader", timeOn).Store()
		attempt += 1
		if attempt < 3 {
			log.Printf("Retrying on request %v", err)
			time.Sleep(1 * time.Second)
		}
	}
}
