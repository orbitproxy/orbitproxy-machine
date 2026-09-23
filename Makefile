VERSION ?= $(shell cat VERSION)
CONTROL_PLANE_URL ?= https://cp.app.orbitproxy.cloud
TLS_VERIFY ?= on
LOCAL_CONTROL_PLANE_URL ?= http://127.0.0.1:3003
BINARY ?= orbitproxy

LDFLAGS := -X github.com/orbitproxy/orbitproxy-machine/buildmeta.ControlPlaneURL=$(CONTROL_PLANE_URL) \
	-X github.com/orbitproxy/orbitproxy-machine/buildmeta.TLSVerify=$(TLS_VERIFY) \
	-X github.com/orbitproxy/orbitproxy-machine/buildmeta.SoftVersion=$(VERSION)

.PHONY: build build-local

build:
	@test -n "$(VERSION)" || (echo "VERSION file is empty" >&2; exit 1)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd

# 本地直连 edge machineListener（TLS 关闭）。版本仍读 VERSION，与发布同一条 ldflags。
build-local:
	$(MAKE) build CONTROL_PLANE_URL=$(LOCAL_CONTROL_PLANE_URL) TLS_VERIFY=off
