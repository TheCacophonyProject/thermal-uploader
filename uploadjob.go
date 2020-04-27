package main

import (
    "bufio"
    "encoding/json"
    "fmt"
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
    uploadMeta  bool
    hasMetaData bool
}

func newUploadJob(filename string) *uploadJob {
    extension := filepath.Ext(filename)
    if extension == ".cptv" {
        exists, name := metaFileExists(filename)
        return &uploadJob{filename: filename, metafile: name, hasMetaData: exists}

    } else if extension == ".txt" {
        _, err := os.Stat(filename)
        exists := err == nil
        var recID int
        if exists {
            recID = getFileRecID(filename)
            log.Printf("got rec id of %v", recID)
        }
        return &uploadJob{filename: "", metafile: filename, hasMetaData: exists, uploadMeta: true, recID: recID}
    }
    return nil
}

func (u *uploadJob) uploadType() string {
    if u.uploadMeta {
        return "metadata"
    }
    return "cptv"
}

func (u *uploadJob) canUploadMeta() bool {
    return u.recID > 0 && u.hasMetaData
}

func (u *uploadJob) uploadName() string {
    if u.uploadMeta {
        return u.metafile
    }
    return u.filename
}

// delete the current file (CPTV or metadata)
func (u *uploadJob) delete() {
    if err := os.Remove(u.uploadName()); err != nil {
        log.Printf("warning: failed to delete %s: %v", u.uploadName(), err)
    }
}

// upload the current file (CPTV or metadata) and delete it on success
func (u *uploadJob) upload(apiClient *api.CacophonyAPI) error {
    var err error
    if u.uploadMeta {
        if !u.canUploadMeta() {
            return fmt.Errorf("Cannot upload %v, recID: %v", u.metafile, u.recID)
        }
        err = uploadMeta(apiClient, u.metafile, u.recID)
    } else {
        u.recID, err = uploadCPTV(apiClient, u.filename)
    }

    if err == nil {
        u.delete()
    }

    if err == nil || os.IsNotExist(err) {
        return nil
    } else {
        return err
    }
}

func (u *uploadJob) renameMeta(toFailedDir bool) error {
    dir, baseName := filepath.Split(u.metafile)
    var newMetaFile string
    if strings.Index(baseName, "-") == -1 && u.recID != 0 {
        newMetaFile = fmt.Sprintf("%v-%v", u.recID, baseName)
    } else {
        newMetaFile = u.metafile
    }
    if toFailedDir {
        return os.Rename(u.metafile, filepath.Join(dir, failedUploadsDir, newMetaFile))
    } else {
        return os.Rename(u.metafile, newMetaFile)

    }
}

func uploadMeta(apiClient *api.CacophonyAPI, filename string, recID int) error {

    baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
    metafile := baseName + ".txt"
    meta, err := loadMeta(metafile)
    if err != nil {
        return err
    }
    meta.RecID = recID
    modelName := meta.ModelName
    if modelName == "" {
        modelName = defaultModel
    }
    model := map[string]interface{}{
        "model": modelName,
    }

    for _, data := range meta.Tracks {
        if data["uploaded"] == true {
            continue
        }
        var tr api.TrackResponse
        tr, err = apiClient.AddTrack(recID, data, meta.Algorithm)

        if err != nil {
            break
        }

        if data["confident_tag"] != nil {
            model["all_class_confidences"] = data["all_class_confidences"]
            model["algorithmId"] = tr.AlgorithmId
            _, err = apiClient.AddTrackTag(recID, tr.TrackID, true, data, model)

            if err != nil {
                break
            }
        }
        data["trackID"] = tr.TrackID
        data["uploaded"] = true
    }
    if err != nil {
        saveMetadata(meta, filename)
        return err
    }
    return nil
}

func uploadCPTV(apiClient *api.CacophonyAPI, filename string) (int, error) {

    f, err := os.Open(filename)
    if err != nil {
        return 0, err
    }

    defer f.Close()
    return apiClient.UploadThermalRaw(bufio.NewReader(f))
}

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

func saveMetadata(meta metadata, filename string) error {

    data, err := json.Marshal(meta)

    if err != nil {
        return err
    }

    return ioutil.WriteFile(filename, data, 0644)

}

type metadata struct {
    RecID     int       `json:"recording_id"`
    ModelName string    `json:"model_name"`
    Algorithm algorithm `json:"algorithm"`
    Tracks    []track   `json:"tracks"`
}

type algorithm map[string]interface{}

type track map[string]interface{}

// moveToFailed moves the cptv and meta file if it exists to the failed uploads directory
// if metadata failed but recording didn't add recid to metadata filename
func (u *uploadJob) moveToFailed() error {
    var errFile error
    if !u.uploadMeta {
        dir, name := filepath.Split(u.filename)
        errFile = os.Rename(u.filename, filepath.Join(dir, failedUploadsDir, name))
    }

    errMeta := u.renameMeta(true)

    if errFile != nil {
        return errFile
    }
    return errMeta
}
