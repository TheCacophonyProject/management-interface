module github.com/TheCacophonyProject/management-interface

go 1.15

require (
	github.com/TheCacophonyProject/audiobait v0.0.0-20191024224540-9caccf222bd2
	github.com/TheCacophonyProject/event-reporter v1.3.2-0.20200210010421-ca3fcb76a231
	github.com/TheCacophonyProject/go-api v0.0.0-20190923033957-174cea2ac81c
	github.com/TheCacophonyProject/go-config v1.6.2
	github.com/TheCacophonyProject/lepton3 v0.0.0-20200909032119-e2b2b778a8ee
	github.com/TheCacophonyProject/rtc-utils v1.2.0
	github.com/TheCacophonyProject/salt-updater v0.2.0
	github.com/gobuffalo/packr v1.30.1
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2
	github.com/nathan-osman/go-sunrise v0.0.0-20201029015502-9a83cd1a5746 // indirect
	github.com/stretchr/testify v1.4.0
	google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
)

replace periph.io/x/periph => github.com/TheCacophonyProject/periph v2.1.1-0.20200615222341-6834cd5be8c1+incompatible

replace github.com/TheCacophonyProject/audiobait => /home/cam/cac/go/audiobait
