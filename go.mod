module github.com/orbitproxy/orbitproxy-machine

go 1.25.8

require (
	github.com/orbitproxy/orbitproxy-go v0.3.8
	github.com/parnurzeal/gorequest v0.2.17
	github.com/spf13/cobra v1.10.2
	golang.org/x/sys v0.47.0
	golang.org/x/term v0.45.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v3 v3.0.1
)

// Local SDK until LoggerOptions + MachineKey Identity are published.

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/net v0.21.0 // indirect
	moul.io/http2curl/v2 v2.3.0 // indirect
)
