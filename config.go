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
<<<<<<< Updated upstream
	"io/ioutil"
	"os"
=======
>>>>>>> Stashed changes

	goconfig "github.com/TheCacophonyProject/go-config"
)

type Config struct {
	Directory string `yaml:"directory"`
}

func ParseConfig(configDir string) (*Config, error) {
	configRW, err := goconfig.New(configDir)
	if err != nil {
		return nil, err
	}

<<<<<<< Updated upstream
func ParseConfigFile(filename string) (*Config, error) {
	buf, err := ioutil.ReadFile(filename)
	if os.IsNotExist(err) {
		return ParseConfig([]byte{})
	} else if err != nil {
=======
	thermalRecorder := goconfig.DefaultThermalRecorder()
	if err := configRW.Unmarshal(goconfig.ThermalRecorderKey, &thermalRecorder); err != nil {
>>>>>>> Stashed changes
		return nil, err
	}

	config := &Config{
		Directory: thermalRecorder.OutputDir,
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (conf *Config) Validate() error {
	if conf.Directory == "" {
		return errors.New("directory missing")
	}
	return nil
}
