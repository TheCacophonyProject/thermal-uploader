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
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/TheCacophonyProject/event-reporter/v3/eventclient"
	"github.com/TheCacophonyProject/go-api"
	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/go-utils/logging"
	"github.com/TheCacophonyProject/modemd/connrequester"
	arg "github.com/alexflint/go-arg"
	"github.com/godbus/dbus"
	"github.com/google/go-cmp/cmp"
	"github.com/rjeczalik/notify"
)

const (
	failedUploadsDir        = "failed-uploads"
	postProcessDir          = "postprocess"
	connectionTimeout       = time.Minute * 2
	connectionRetryInterval = time.Minute * 10
	failedRetryInterval     = time.Minute * 10
	failedRetryMaxInterval  = time.Hour * 24
)

var log = logging.NewLogger("info")
var version = "No version provided"
var globs = [6]string{"*.cptv", "*.avi", "*.mp4", "*.aac", "*.wav", "*.m4a"}

type Args struct {
	ConfigDir string `arg:"-c,--config" help:"path to configuration directory"`
	logging.LogArgs
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

// getGlobMatches returns a map of glob patterns to their matched file type.
func getGlobMatches(directory string) map[string][]string {
	var matches = make(map[string][]string)
	for _, glob := range globs {
		globMatches, _ := filepath.Glob(filepath.Join(directory, glob))

		matches[glob] = globMatches
	}
	return matches
}

func makeFileMetricEvents(directory string) {
	nextEventTime := time.Now()
	for {

		// Get a map of all the files, including failed uploads.
		readyToUploadMatches := getGlobMatches(directory)
		failedUploadMatches := getGlobMatches(filepath.Join(directory, failedUploadsDir))
		postProcessMatches := getGlobMatches(filepath.Join(directory, postProcessDir))
		allMatches := map[string][]string{}
		for glob, files := range readyToUploadMatches {
			allMatches[strings.TrimPrefix(glob, "*.")] = files
		}
		for glob, files := range failedUploadMatches {
			allMatches[strings.TrimPrefix(glob, "*.")+"-failed"] = files
		}
		for glob, files := range postProcessMatches {
			allMatches[strings.TrimPrefix(glob, "*.")+"-postprocessed"] = files
		}

		// Count the files and total size.
		matchFileCount := map[string]any{}
		totalFileCount := 0
		totalFileSizeKB := 0
		for fileType, files := range allMatches {
			// Skip globs with no matches
			if len(files) == 0 {
				continue
			}
			matchFileCount[fileType] = len(files)
			totalFileCount += len(files)
			for _, file := range files {
				fileInfo, err := os.Stat(file)
				if err != nil {
					// Files are getting deleted/moved at the same time so we can ignore this error.
					continue
				}
				totalFileSizeKB += int(fileInfo.Size() / 1024)
			}
		}

		// Find free space on the disk.
		var freeSpaceKB int
		var stat syscall.Statfs_t
		if err := syscall.Statfs("/", &stat); err != nil {
			log.Errorf("Failed to get disk free space: %v", err)
		} else {
			freeSpaceKB = int(stat.Bavail * uint64(stat.Bsize) / 1024)
		}

		// Make/Add event logging the files count, total count, and file size.
		if err := eventclient.AddEvent(eventclient.Event{
			Timestamp: time.Now(),
			Type:      "fileCount",
			Details: map[string]any{
				"fileCount":       matchFileCount,
				"totalFileCount":  totalFileCount,
				"totalFileSizeKB": totalFileSizeKB,
				"freeSpaceKB":     freeSpaceKB,
			},
		}); err != nil {
			log.Errorf("Failed to make file count event: %v", err)
		}
		log.Infof("Logged file count event. Total file count: %v", totalFileCount)

		// Wait to make the next metric event
		nextEventTime = nextEventTime.Add(time.Hour * 24)
		time.Sleep(time.Until(nextEventTime))
	}
}

func runMain() error {
	args := procArgs()

	log = logging.NewLogger(args.LogLevel)

	log.Printf("running version: %s", version)

	cr := connrequester.NewConnectionRequester()
	log.Println("requesting internet connection")
	cr.Start()
	err := cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
	if err != nil {
		return err
	}
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
	go checkConfigChanges(conf, args.ConfigDir)

	go makeFileMetricEvents(conf.Directory)

	log.Println("Making failed uploads directory")
	{
		err := os.MkdirAll(filepath.Join(conf.Directory, failedUploadsDir), 0755)
		if err != nil {
			return err
		}
	}

	log.Println("Watching", conf.Directory)
	fsEvents := make(chan notify.EventInfo, 1)
	if err := notify.Watch(conf.Directory, fsEvents, notify.InCloseWrite, notify.InMovedTo); err != nil {
		return err
	}
	defer notify.Stop(fsEvents)

	nextFailedRetry := time.Now()
	failedRetryAttempts := 0

	go updateStayOnLoop()

	for {
		// Ask tc2-hat-attiny to keep the device on while uploading. This is
		// only a request, uploading continues if it fails.
		setBusyUploading(true)

		// Check for files to upload first in case there are CPTV
		// files around when the uploader starts.
		cr.Start()
		{
			err := cr.WaitUntilUpLoop(connectionTimeout, connectionRetryInterval, -1)
			if err != nil {
				return err
			}
		}
		if err = uploadFiles(apiClient, conf.Directory); err != nil {
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

		// Check if we can stop or if there is a new file to be uploaded.
		select {
		case <-fsEvents:
			// A new file was added during the last iteration, loop again.
		case <-time.After(time.Second):
			// No new file was added, then:
			setBusyUploading(false) // Tell tc2-hat-attiny that we are all done.
			cr.Stop()               // Stop requesting an internet connection.
			<-fsEvents              // Wait for a new file to be added.
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a > b {
		return b
	} else {
		return a
	}
}

func uploadFiles(apiClient *api.CacophonyAPI, directory string) error {
	var globMatches = getGlobMatches(directory)
	var matches = make([]string, 0)
	for _, files := range globMatches {
		matches = append(matches, files...)
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
			err := job.moveToFailed()
			if err != nil {
				return err
			}
			continue
		}
		err = uploadFileWithRetries(apiClient, job)
		if err != nil {
			return err
		}
	}
	return nil
}

func retryFailedUploads(apiClient *api.CacophonyAPI, directory string) bool {
	// Get the files that failed previously
	var matchesMap = getGlobMatches(filepath.Join(directory, failedUploadsDir))
	var matches = make([]string, 0)
	for _, files := range matchesMap {
		matches = append(matches, files...)
	}
	if len(matches) == 0 {
		return true
	}

	// start at a random index to avoid always failing on the same file
	startIndex := rand.Intn(len(matches))
	var urlError *url.Error

	// Try to upload the files that failed previously.
	for i := 0; i < len(matches); i++ {
		index := (startIndex + i) % len(matches)
		filename := matches[index]
		job := newUploadJob(filename)
		err := job.upload(apiClient)

		if err != nil {
			log.Printf("Uploading still failing to upload %v: %v", filename, err)

			// any http request error will be caught here, I think if not http error should try all other files
			if errors.As(err, &urlError) {
				return false
			}
			continue
		}
		log.Print("Success in uploading previously failed file: ", filename)
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

// checkConfigChanges will compare the config from when first loaded to a new config each time
// the config file is modified.
// If there is a difference then the program will exit and systemd will restart the service, causing
// the new config to be loaded.
func checkConfigChanges(conf *Config, configDir string) error {
	configFilePath := filepath.Join(configDir, goconfig.ConfigFileName)
	fsEvents := make(chan notify.EventInfo, 1)
	if err := notify.Watch(configFilePath, fsEvents, notify.InCloseWrite, notify.InMovedTo); err != nil {
		return err
	}
	defer notify.Stop(fsEvents)

	for {
		<-fsEvents
		newConfig, err := ParseConfig(configDir)
		log.Debug("New config:", newConfig)

		if err != nil {
			log.Error("error reloading config:", err)
			continue
		}
		diff := cmp.Diff(conf, newConfig)
		log.Debug("Config diff:", diff)
		if diff != "" {
			log.Info("Config changed. Exiting to allow systemctl to restart service.")
			os.Exit(0)
		} else {
			log.Info("No relevant changes detected in config file.")
		}
	}
}

const dbusDest = "org.cacophony.ATtiny"
const dbusPath = "/org/cacophony/ATtiny"

const stayOnMinutes = int64(3)           // Each request will be for 3 minutes
const stayOnUpdateInterval = time.Minute // Send an update every minute

var (
	stayOnBusy        atomic.Bool              // Set when the device needs to stay on for uploading.
	stayOnUpdate      = make(chan struct{}, 1) // Triggers an update without waiting for the interval.
	attinyUnavailable bool                     // Only used by updateStayOnLoop, so needs no locking.
)

func getDbusObj() (dbus.BusObject, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	obj := conn.Object(dbusDest, dbusPath)
	return obj, nil
}

func isServiceUnknown(err error) bool {
	var dbusErr dbus.Error
	return errors.As(err, &dbusErr) && dbusErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown"
}

func setBusyUploading(busy bool) {
	if stayOnBusy.Swap(busy) != busy { // If old value != new value
		if busy {
			log.Println("Requesting to stay on for uploading")
		} else {
			log.Println("Uploading finished, no longer requesting to stay on")
		}
	}

	select {
	case stayOnUpdate <- struct{}{}:
	default: // An update is already queued.
	}
}

func updateStayOnLoop() {
	for {
		select {
		case <-stayOnUpdate:
		case <-time.After(stayOnUpdateInterval):
		}
		sendStayOnState()
	}
}

func sendStayOnState() {
	obj, err := getDbusObj()
	if err != nil {
		log.Warnf("Failed to connect to dbus: %v", err)
		return
	}

	busy := stayOnBusy.Load()
	if busy {
		err = obj.Call(dbusDest+".StayOnForProcess", 0, "uploader", stayOnMinutes).Store()
	} else {
		err = obj.Call(dbusDest+".StayOnFinished", 0, "uploader").Store()
	}
	if err != nil {
		if isServiceUnknown(err) {
			if !attinyUnavailable {
				attinyUnavailable = true
				log.Warnf("%s service is not running, continuing without it", dbusDest)
			}
			return
		}
		log.Warnf("Failed to update stay on state with the ATtiny: %v", err)
		return
	}
	attinyUnavailable = false
	log.Debugf("Sent stay on state of %v to the ATtiny", busy)
}
