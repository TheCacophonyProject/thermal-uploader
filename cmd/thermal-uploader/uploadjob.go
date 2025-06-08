package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	api "github.com/TheCacophonyProject/go-api"
)

var timeLayouts = [3]string{"2006-01-02--15-04-05", "20060102-150405.000000", "20060102-150405"}

type uploadJob struct {
	filename    string
	metafile    string
	recID       int
	hasMetaData bool
	duration    int
	dateParsed  bool
	recDate     time.Time
}

func (u *uploadJob) requiresConversion() bool {
	return filepath.Ext(u.filename) == ".avi" || filepath.Ext(u.filename) == ".wav"
}

func (u *uploadJob) isIR() bool {
	return filepath.Ext(u.filename) == ".avi" || filepath.Ext(u.filename) == ".mp4"
}

func (u *uploadJob) isAudio() bool {
	return filepath.Ext(u.filename) == ".wav" || filepath.Ext(u.filename) == ".aac"
}

func (u *uploadJob) isThermal() bool {
	return filepath.Ext(u.filename) == ".cptv"
}

func (u *uploadJob) fileType() string {
	fileType := "thermalRaw"
	if u.isIR() {
		fileType = "irRaw"
	} else if u.isAudio() {
		fileType = "audio"
	}
	return fileType
}

func (u *uploadJob) convert() error {
	if !u.requiresConversion() {
		return nil
	} else if u.isAudio() {
		return u.convertAudio()
	} else if u.isIR() {
		return u.convertMp4()
	}
	return nil
}

func (u *uploadJob) convertAudio() error {
	var extension = filepath.Ext(u.filename)

	var name = u.filename[0:len(u.filename)-len(extension)] + ".aac"
	var duration = fmt.Sprintf("duration=%d", u.duration)
	args := []string{"-y", // Yes to all
		"-i", u.filename,
		"-codec:a", "aac",
		"-b:a", "128k",
		"-q:a",
		"1.2",
		"-aac_coder",
		"fast",
		"-movflags",
		"faststart",
		"-movflags",
		"+use_metadata_tags",
		"-map_metadata",
		"0"}

	if u.dateParsed {
		var recDateTime = fmt.Sprintf("recordingDateTime=%s", u.recDate.Format(time.RFC3339))
		args = append(args,
			"-metadata",
			recDateTime)
	}
	args = append(args,
		"-metadata",
		duration,
		"-f",
		"mp4", name)

	cmd := exec.Command("ffmpeg", args...)
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
func metaFileExists(filename string) (bool, string) {
	metafile := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".txt"
	if _, err := os.Stat(metafile); err != nil {
		return false, ""
	}
	return true, metafile
}

func newUploadJob(filename string) *uploadJob {
	exists, name := metaFileExists(filename)
	u := &uploadJob{filename: filename, metafile: name, hasMetaData: exists, duration: 0, dateParsed: false}
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

func (u *uploadJob) parseDateTime() error {
	for _, layout := range timeLayouts {
		dt, err := parseDateTime(u.filename, layout, false)
		if err == nil {
			u.recDate = dt
			u.dateParsed = true
			break
		}
	}
	return fmt.Errorf("Could not parse date time")
}
func (u *uploadJob) preprocess() error {
	err := u.parseDateTime()
	if err != nil {
		log.Printf("Error getting datetime %v\n", err)
	}
	err = u.setDuration()
	if err != nil {
		log.Printf("Error getting duration %v\n", err)
	}
	return u.convert()
}

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

func (u *uploadJob) setDuration() error {
	if u.isThermal() {
		return nil
	}
	if u.isAudio() && !u.requiresConversion() {
		return nil
	}

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
		return err
	}
	outString := strings.TrimSuffix(string(out), "\n")
	i, err := strconv.ParseFloat(outString, 32)

	u.duration = int(i)

	return err
}

// upload the current file (CPTV or metadata) and delete it on success
func (u *uploadJob) upload(apiClient *api.CacophonyAPI) error {
	var err error
	u.recID, err = u.uploadFile(apiClient)
	if err == nil {
		u.delete()
	}

	if err == nil || os.IsNotExist(err) {
		return nil
	} else {
		return err
	}
}

func (u *uploadJob) uploadFile(apiClient *api.CacophonyAPI) (int, error) {
	var meta metadata
	var err error
	if u.hasMetaData {
		meta, err = loadMeta(u.metafile)
		if err != nil {
			log.Printf("Error loading metadata %v\n", err)
		}
	}
	data := map[string]interface{}{
		"type": u.fileType(),
	}
	if u.isIR() || u.isAudio() {
		if u.dateParsed {
			data["recordingDateTime"] = u.recDate.Format(time.RFC3339)
		}
	}
	if u.duration > 0 {
		data["duration"] = u.duration
	}
	if meta != nil {
		data["metadata"] = meta
	}
	data["filename"] = u.filename

	err = u.convert()
	if err != nil {
		return 0, err
	}
	f, err := os.Open(u.filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return apiClient.UploadVideo(bufio.NewReader(f), data)
}

func parseDateTime(filename string, layout string, utctime bool) (time.Time, error) {
	file := filepath.Base(filename)
	file = strings.TrimSuffix(file, filepath.Ext(file))
	// var additionalMetadata = make(map[string]interface{})

	// GP this will change
	if len(file) >= len(layout) {
		file = file[:len(layout)]
		// attempt to get system timezone
		var t time.Time
		var err error
		if utctime {
			t, err = time.Parse(layout, file)
		} else {
			loc, err := time.LoadLocation("Local")
			if err != nil {
				log.Printf("Could not get local location%v\n", err)
				t, err = time.Parse(layout, file)
				if err != nil {
					log.Errorf("Could not parse date time for %v %v\n", filename, err)
					return time.Time{}, err
				}
			} else {
				t, err = time.ParseInLocation(layout, file, loc)
				if err != nil {
					log.Errorf("Could not parse date time for %v %v\n", filename, err)
					return time.Time{}, err
				}
				log.Printf("Parsed location %v %v %v", file, loc, t)
			}
		}
		if err != nil {
			log.Printf("Could not parse date time for %v %v\n", filename, err)
			return time.Time{}, err
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse date time for %v with expected layout %v", filename, layout)
}

type metadata map[string]interface{}

func loadMeta(filename string) (metadata, error) {
	var meta metadata
	byteValue, err := os.ReadFile(filename)
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
