module github.com/TheCacophonyProject/management-interface

go 1.17

require (
	github.com/TheCacophonyProject/audiobait/v3 v3.0.1
	github.com/TheCacophonyProject/event-reporter v1.3.2-0.20200210010421-ca3fcb76a231
	github.com/TheCacophonyProject/go-api v1.0.0
	github.com/TheCacophonyProject/go-config v1.7.0
	github.com/TheCacophonyProject/go-cptv v0.0.0-20201215230510-ae7134e91a71
	github.com/TheCacophonyProject/lepton3 v0.0.0-20210324024142-003e5546e30f
	github.com/TheCacophonyProject/rtc-utils v1.2.0
	github.com/TheCacophonyProject/salt-updater v0.4.0
	github.com/gobuffalo/packr v1.30.1
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/gorilla/mux v1.8.0
	github.com/nathan-osman/go-sunrise v1.0.0 // indirect
	golang.org/x/net v0.0.0-20210927181540-4e4d966f7476
)

require (
	github.com/TheCacophonyProject/event-reporter/v3 v3.3.0 // indirect
	github.com/TheCacophonyProject/window v0.0.0-20200312071457-7fc8799fdce7 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gobuffalo/envy v1.9.0 // indirect
	github.com/gobuffalo/packd v1.0.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/joho/godotenv v1.4.0 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.9.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	golang.org/x/sys v0.0.0-20211006225509-1a26e0398eed // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	periph.io/x/periph v3.6.8+incompatible // indirect
)

replace github.com/TheCacophonyProject/thermal-recorder => /home/gp/cacophony/thermal-recorder

replace periph.io/x/periph => github.com/TheCacophonyProject/periph v2.1.1-0.20200615222341-6834cd5be8c1+incompatible
