module github.com/TheCacophonyProject/thermal-uploader

go 1.15

require (
	github.com/TheCacophonyProject/event-reporter/v3 v3.4.0 // indirect
	github.com/TheCacophonyProject/go-api v1.0.3
	github.com/TheCacophonyProject/go-config v1.8.2
	github.com/TheCacophonyProject/modemd v1.1.1
	github.com/alexflint/go-arg v1.4.2
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/pelletier/go-toml/v2 v2.0.2 // indirect
	github.com/rjeczalik/notify v0.0.0-20171004161231-1aa3b9de8d84
	github.com/spf13/afero v1.9.2
	github.com/spf13/viper v1.12.0 // indirect
	github.com/stretchr/testify v1.7.2
	github.com/subosito/gotenv v1.4.0 // indirect
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b // indirect
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
	gopkg.in/ini.v1 v1.66.6 // indirect
	periph.io/x/periph v3.7.0+incompatible // indirect
)

replace periph.io/x/periph => github.com/TheCacophonyProject/periph v2.1.1-0.20200615222341-6834cd5be8c1+incompatible
