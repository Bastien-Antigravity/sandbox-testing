module github.com/Bastien-Antigravity/testing-sandbox/scenarios

go 1.25.8

require (
	capnproto.org/go/capnp/v3 v3.1.0-alpha.2
	github.com/Bastien-Antigravity/distributed-config v1.9.922
	github.com/Bastien-Antigravity/flexible-logger v1.3.3
	github.com/Bastien-Antigravity/microservice-toolbox v0.0.0
	github.com/Bastien-Antigravity/notif-server v1.1.5
	github.com/Bastien-Antigravity/safe-socket v1.8.2
	github.com/Bastien-Antigravity/universal-logger v1.4.2
	github.com/lib/pq v1.11.2
	github.com/nats-io/nats.go v1.52.0
	github.com/stretchr/testify v1.11.1
	google.golang.org/protobuf v1.36.11
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
	modernc.org/sqlite v1.52.0
)

require (
	github.com/colega/zeropool v0.0.0-20230505084239-6fb4a4f75381 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260523011958-0a33c5d7ca68 // indirect
	google.golang.org/grpc v1.81.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace github.com/Bastien-Antigravity/microservice-toolbox => ../../../microservice-toolbox

replace github.com/Bastien-Antigravity/flexible-logger => ../../../flexible-logger

replace github.com/Bastien-Antigravity/safe-socket => ../../../safe-socket
