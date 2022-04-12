package main

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	api "github.com/TheCacophonyProject/go-api"
)

type uploadJob struct {
	filename    string
	metafile    string
	recID       int
	duration    int
	hasMetaData bool
	avi         bool
}

func metaFileExists(filename string) (bool, string) {
	metafile := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".txt"
	if _, err := os.Stat(metafile); err != nil {
		return false, ""
	}
	return true, metafile
}

func newUploadJob(filename string) *uploadJob {
	exists, name := metaFileExists(filename)
	avi := filepath.Ext(filename) == ".avi"
	u := &uploadJob{filename: filename, metafile: name, hasMetaData: exists, avi: avi}
	return u
}

// delete the current file (CPTV or metadata)
func (u *uploadJob) delete() {
	if err := os.Remove(u.filename); err != nil {
		log.Printf("warning: failed to delete %s: %v", u.filename, err)
	}
	if u.hasMetaData {
		if err := os.Remove(u.metafile); err != nil {
			log.Printf("warning: failed to delete %s: %v", u.metafile, err)
		}
	}
}

// ffmpegConversion        = "ffmpeg -i %s -c:v copy -c:a copy -y %s"

func (u *uploadJob) convertMp4() error {
	var extension = filepath.Ext(u.filename)
	var name = u.filename[0:len(u.filename)-len(extension)] + ".mp4"
	cmd := exec.Command("ffmpeg", "-y", // Yes to all
		"-i", u.filename,
		"-map_metadata", "-1", // strip out all (mostly) metadata
		"-vcodec", "libx264",
		"-c:a", "copy",
		name,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err := cmd.Run()
	if err != nil {
		return err
	}
	if err := os.Remove(u.filename); err != nil {
		log.Printf("warning: failed to delete %s: %v", u.filename, err)
	}

	u.filename = name
	return nil
}

func (u *uploadJob) getDuration() (int, error) {
	// ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 input.mp4
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", // strip out all (mostly) metadata
		u.filename,
	)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		log.Printf("error getting duration %v", err)
		return 0, err
	}
	outString := strings.TrimSuffix(string(out), "\n")
	i, err := strconv.ParseFloat(outString, 16)
	return int(i), err
}

func (u *uploadJob) preprocess() error {
	if u.avi {
		err := u.convertMp4()
		if err != nil {
			u.moveToFailed()
			return err
		}
		dur, err := u.getDuration()
		if err == nil {
			u.duration = dur
		}
	}
	return nil
}

// upload the current file (CPTV or metadata) and delete it on success
func (u *uploadJob) upload(apiClient *api.CacophonyAPI) error {
	var err error
	u.recID, err = u.uploadCPTV(apiClient)
	if err == nil {
		u.delete()
	}

	if err == nil || os.IsNotExist(err) {
		return nil
	} else {
		return err
	}
}

func (u *uploadJob) uploadCPTV(apiClient *api.CacophonyAPI) (int, error) {
	var meta metadata
	var err error
	if u.hasMetaData {
		meta, err = loadMeta(u.metafile)
		if err != nil {
			log.Printf("Error loading metadata %v\n", err)
		}
	}
	vidType := "thermalRaw"
	if u.avi {
		vidType = "irRaw"
	}
	data := map[string]interface{}{
		"type": vidType,
	}
	if u.avi {
		file := filepath.Base(u.filename)
		// GP this will change
		const layout = "2006-01-02_15.04.05"
		file = file[:len(layout)]
		t, err := time.Parse(layout, file)
		if err != nil {
			log.Printf("Coul not parse date time for %v %v", u.filename, err)
		} else {
			data["recordingDateTime"] = t
		}
	}

	if u.duration > 0 {
		data["duration"] = u.duration
	}
	if meta != nil {
		data["metadata"] = meta
	}

	f, err := os.Open(u.filename)
	defer f.Close()
	return apiClient.UploadThermalRaw(bufio.NewReader(f), data)
}

type metadata map[string]interface{}

func loadMeta(filename string) (metadata, error) {
	var meta metadata
	byteValue, err := ioutil.ReadFile(filename)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(byteValue, &meta); err != nil {
		log.Printf("Could not parse metadata %v\n", err)
		return meta, err
	}
	return meta, nil
}

// moveToFailed moves the cptv and meta file if it exists to the failed uploads directory
func (u *uploadJob) moveToFailed() error {
	var errFile, errMeta error
	dir, name := filepath.Split(u.filename)
	errFile = os.Rename(u.filename, filepath.Join(dir, failedUploadsDir, name))

	if u.hasMetaData {
		dir, baseName := filepath.Split(u.metafile)
		errMeta = os.Rename(u.metafile, filepath.Join(dir, failedUploadsDir, baseName))
	}

	if errFile != nil {
		return errFile
	}
	return errMeta
}
