// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type CacophonyAPI struct {
	ServerURL  string
	Group      string
	DeviceName string
	Password   string
	Token      string
}

func (api *CacophonyAPI) Register() error {
	if api.Password != "" {
		return errors.New("already registered")
	}

	password := randString(20)
	payload, err := json.Marshal(map[string]string{
		"group":      api.Group,
		"devicename": api.DeviceName,
		"password":   password,
	})
	if err != nil {
		return err
	}
	resp, err := http.Post(
		api.ServerURL+"/api/v1/devices",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respData := struct {
		Success  bool
		Messages []string
		Token    string
	}{}
	d := json.NewDecoder(resp.Body)
	if err := d.Decode(&respData); err != nil {
		return fmt.Errorf("decode: %v", err)
	}

	if !respData.Success {
		reason := "unknown"
		if len(respData.Messages) > 0 {
			reason = respData.Messages[0]
		}
		return fmt.Errorf("registration failed: %v", reason)
	}

	api.Password = password
	api.Token = respData.Token
	return nil
}
