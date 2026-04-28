module github.com/Bastien-Antigravity/testing-sandbox/scenarios

go 1.25.4

require (
	github.com/Bastien-Antigravity/microservice-toolbox/go v1.1.7
	github.com/Bastien-Antigravity/universal-logger v0.0.0
	github.com/stretchr/testify v1.11.1
)

require (
	capnproto.org/go/capnp/v3 v3.1.0-alpha.2 // indirect
	github.com/Bastien-Antigravity/distributed-config v1.9.0 // indirect
	github.com/Bastien-Antigravity/flexible-logger v1.3.0 // indirect
	github.com/Bastien-Antigravity/safe-socket v1.8.0 // indirect
	github.com/colega/zeropool v0.0.0-20230505084239-6fb4a4f75381 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/Bastien-Antigravity/distributed-config => ../../distributed-config

replace github.com/Bastien-Antigravity/universal-logger => ../../universal-logger

replace github.com/Bastien-Antigravity/microservice-toolbox/go => ../../microservice-toolbox/go

replace github.com/Bastien-Antigravity/safe-socket => ../../safe-socket

replace github.com/Bastien-Antigravity/flexible-logger => ../../flexible-logger
