package main

import (
	"path"
	"testing"

	goconfig "github.com/TheCacophonyProject/go-config"

	"github.com/TheCacophonyProject/go-config/configtest"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestBadConfigFile(t *testing.T) {
	defer newFs(t, "./test-files/bad-config.toml")()
	_, err := ParseConfig(goconfig.DefaultConfigDir)
	require.Error(t, err)
}

func TestDefaultConfig(t *testing.T) {
	defer newFs(t, "")()
	conf, err := ParseConfig(goconfig.DefaultConfigDir)
	require.NoError(t, err)
	require.Equal(t, conf.Directory, goconfig.DefaultThermalRecorder().OutputDir)
}

func newFs(t *testing.T, configFile string) func() {
	fs := afero.NewMemMapFs()
	goconfig.SetFs(fs)
	fsConfigFile := path.Join(goconfig.DefaultConfigDir, goconfig.ConfigFileName)
	lockFileFunc, cleanupFunc := configtest.WriteConfigFromFile(t, configFile, fsConfigFile, fs)
	goconfig.SetLockFilePath(lockFileFunc)
	return cleanupFunc
}
