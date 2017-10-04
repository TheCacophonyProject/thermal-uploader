// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"fmt"
)

// XXX to come from YAML config
const (
	serverURL  = "http://127.0.0.1:9999"
	group      = "foo"
	deviceName = "foo3"
	password   = "5iwhqm7qfvylupi6jxn2"
)

func main() {
	api, err := NewAPI(serverURL, group, deviceName, password)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("password", api.Password())
	fmt.Println("token", api.token)
}
