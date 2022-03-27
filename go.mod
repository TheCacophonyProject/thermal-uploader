module github.com/TheCacophonyProject/thermal-uploader

go 1.15

replace github.com/TheCacophonyProject/go-api => /home/gp/cacophony/go-api

require (
	github.com/TheCacophonyProject/go-api v0.0.0-20210601232725-ff622379ffa0
	github.com/TheCacophonyProject/go-config v1.7.0
	github.com/TheCacophonyProject/modemd v1.1.1
	github.com/alexflint/go-arg v1.4.2
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/rjeczalik/notify v0.0.0-20171004161231-1aa3b9de8d84
	github.com/spf13/afero v1.6.0
	github.com/stretchr/testify v1.7.0
)

replace periph.io/x/periph => github.com/TheCacophonyProject/periph v2.1.1-0.20200615222341-6834cd5be8c1+incompatible
