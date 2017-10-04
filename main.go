// Copyright 2017 The Cacophony Project. All rights reserved.

package main

import (
	"fmt"
)

// XXX to come from YAML config
const (
	serverURL = "http://127.0.0.1:9999"
	group     = "foo"
)

func main() {
	api := &CacophonyAPI{
		ServerURL:  serverURL,
		Group:      group,
		DeviceName: "foo2",
	}

	if err := api.Register(); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("password", api.Password)
	fmt.Println("token", api.Token)
}
