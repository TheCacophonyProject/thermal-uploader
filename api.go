// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"time"
)

const timeout = 30 * time.Second

// NewAPI creates a CacophonyAPI instance and obtains a fresh JSON Web
// Token. If no password is given then the device is registered.
func NewAPI(serverURL, group, deviceName, userName, password string) (*CacophonyAPI, error) {
	api := &CacophonyAPI{
		serverURL: serverURL,
		group:     group,
		password:  password,
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   timeout, // connection timeout
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,

				TLSHandshakeTimeout:   timeout,
				ResponseHeaderTimeout: timeout,
				ExpectContinueTimeout: 1 * time.Second,

				MaxIdleConns:    5,
				IdleConnTimeout: 90 * time.Second,
			},
		},
	}
	api.isDevice = deviceName != ""
	if api.isDevice {
		api.name = deviceName
		api.typeName = "devicename"
		api.regURL = api.serverURL + "/api/v1/devices"
		api.authURL = api.serverURL + "/authenticate_device"
	} else {
		api.name = userName
		api.typeName = "username"
		api.regURL = api.serverURL + "/api/v1/users"
		api.authURL = api.serverURL + "/authenticate_user"
	}
	if password == "" {
		err := api.register()
		if err != nil {
			return nil, err
		}
		api.justRegistered = true
	} else {
		err := api.newToken()
		if err != nil {
			return nil, err
		}
	}
	return api, nil
}

type CacophonyAPI struct {
	serverURL      string
	group          string
	name           string
	typeName       string
	regURL         string
	authURL        string
	password       string
	token          string
	justRegistered bool
	isDevice       bool

	client *http.Client
}

func (api *CacophonyAPI) Password() string {
	return api.password
}

func (api *CacophonyAPI) JustRegistered() bool {
	return api.justRegistered
}

func (api *CacophonyAPI) register() error {
	if api.password != "" {
		return errors.New("already registered")
	}

	password := randString(20)
	payload, err := json.Marshal(map[string]string{
		"group":      api.group,
		api.typeName: api.name,
		"password":   password,
	})
	if err != nil {
		return err
	}
	postResp, err := api.client.Post(
		api.regURL,
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	var respData tokenResponse
	d := json.NewDecoder(postResp.Body)
	if err := d.Decode(&respData); err != nil {
		return fmt.Errorf("decode: %v", err)
	}
	if !respData.Success {
		return fmt.Errorf("registration failed: %v", respData.message())
	}

	api.password = password
	api.token = respData.Token
	return nil
}

func (api *CacophonyAPI) newToken() error {
	if api.password == "" {
		return errors.New("no password set")
	}
	payload, err := json.Marshal(map[string]string{
		api.typeName: api.name,
		"password":   api.password,
	})
	if err != nil {
		return err
	}
	postResp, err := api.client.Post(
		api.authURL,
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	var resp tokenResponse
	d := json.NewDecoder(postResp.Body)
	if err := d.Decode(&resp); err != nil {
		return fmt.Errorf("decode: %v", err)
	}
	if !resp.Success {
		return fmt.Errorf("failed getting new token: %v", resp.message())
	}
	api.token = resp.Token
	return nil
}

func (api *CacophonyAPI) getPOSTUrl(devicename string) string {
	if api.isDevice {
		return api.serverURL + "/api/v1/recordings"
	}
	return api.serverURL + "/api/v1/recordings/" + devicename
}

func (api *CacophonyAPI) UploadThermalRaw(info *cptvInfo, r io.Reader) error {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	// JSON encoded "data" parameter.
	dataBuf, err := json.Marshal(map[string]string{
		"type":              "thermalRaw",
		"duration":          strconv.Itoa(info.duration),
		"recordingDateTime": info.timestamp.Format("2006-01-02 15:04:05-0700"),
	})
	if err != nil {
		return err
	}
	if err := w.WriteField("data", string(dataBuf)); err != nil {
		return err
	}

	// Add the file as a new MIME part.
	fw, err := w.CreateFormFile("file", "file")
	if err != nil {
		return err
	}
	io.Copy(fw, r)

	w.Close()

	req, err := http.NewRequest("POST", api.getPOSTUrl(info.devicename), buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", api.token)

	resp, err := api.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		bodyString := string(bodyBytes)
		log.Printf("status code: %d, body:\n%s", resp.StatusCode, bodyString)
		return errors.New("non 200 status code")
	}
	return nil
}

type tokenResponse struct {
	Success  bool
	Messages []string
	Token    string
}

func (r *tokenResponse) message() string {
	if len(r.Messages) > 0 {
		return r.Messages[0]
	}
	return "unknown"
}
