package main

import (
    "bufio"
    "encoding/json"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"
    "strings"

    api "github.com/TheCacophonyProject/go-api"
)

type uploadJob struct {
    filename    string
    metafile    string
    recID       int
    hasMetaData bool
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
  return &uploadJob{filename: filename, metafile: name, hasMetaData: exists}
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

func (u *uploadJob)uploadCPTV(apiClient *api.CacophonyAPI) (int, error) {
  var meta metadata;
  var err error;
  if u.hasMetaData {
    meta, err = loadMeta(u.metafile)
    if err != nil {
      log.Printf("Error loading metadata %v\n", err)
    }
  }

    f, err := os.Open(u.filename)
    if err != nil {
        return 0, err
    }

    defer f.Close()
    return apiClient.UploadThermalRaw(bufio.NewReader(f), meta)
}

type metadata  map[string]interface{}

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
