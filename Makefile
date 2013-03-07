PREFIX=/usr/local
DESTDIR=
BINDIR=${PREFIX}/bin
DATADIR=${PREFIX}/share
GOFLAGS?=
GOTAGS?=

HAUNTS_SRCS = $(wildcard *.go base/*.go game/*.go game/actions/*.go game/ai/*.go game/hui/*.go game/status/*.go house/*.go mrgnet/*.go sound/*.go texture/*.go )
TOOLS_SRCS = $(wildcard tools/*.go)

all: haunts

# Dependencies
haunts: GEN_version.go $(HAUNTS_SRCS)
	@echo Building $@
	go build -o $@ $(GOFLAGS)

haunts_release: GEN_version.go $(HAUNTS_SRCS)
	@echo Building $@
	go build -o $@ -tags 'release !nosound $(GOTAGS)' $(GOFLAGS)

haunts_nosound: GEN_version.go $(HAUNTS_SRCS)
	@echo Building $@
	go build -o $@ -tags 'nosound !release $(GOTAGS)' $(GOFLAGS)

tools/tools: $(tools_SRCS)
	@echo Building $@
	cd $(dir $@) && go build $(GOFLAGS)

GEN_version.go: tools/tools
	@echo Generating $@
	cd tools && ./tools

clean:
	rm -fr tools/tools haunts GEN_version.go

# Targets
.PHONY: install clean all

install: $(BINARIES)
