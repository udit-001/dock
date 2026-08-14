# App Store — common tasks.
# Note: Fyne/GLFW needs the unversioned X11 dev symlink (libXxf86vm.so) which
# this machine lacks; `deps` creates a local one and sets CGO_LDFLAGS.

APP      := app-store
BIN     := bin
OUT     := $(BIN)/$(APP)
LOCALLIB := $(HOME)/.local/lib
export CGO_LDFLAGS = -L$(LOCALLIB)

VERSION_MAJOR := 0
VERSION_MINOR := 0
VERSION_PATCH := 1
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X github.com/udit-001/app-store/internal/version.Version=$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH) \
	-X github.com/udit-001/app-store/internal/version.Commit=$(GIT_COMMIT) \
	-X github.com/udit-001/app-store/internal/version.Date=$(BUILD_DATE)

.PHONY: all deps build run test vet tidy gen install winres version inspect screenshot clean

all: build

# Local dev symlink for the missing unversioned Xlib dev lib.
deps:
	@mkdir -p $(LOCALLIB)
	@ln -sf /lib64/libXxf86vm.so.1 $(LOCALLIB)/libXxf86vm.so
	@go get gopkg.in/yaml.v3@latest fyne.io/fyne/v2@latest

build: deps
	@mkdir -p $(BIN)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT) ./cmd/$(APP)

# View the desktop app. (Off-screen captures aren't shown to the user; run this
# on a machine with a display.)
run: build
	$(OUT)

screenshot:
	@echo "View the app with 'make run' on a display. Off-screen capture isn't supported."

test:
	go test ./...

vet:
	go vet ./...

# Dump the widget-tree markup (Playwright-style DOM snapshot) of the UI.
inspect: build
	./bin/$(APP) -inspect -

tidy:
	go mod tidy

# Regenerate apps.json from the embedded manifest (future jsDelivr path).
gen:
	go run ./cmd/manifest-gen -manifest internal/appdata/manifest.yaml

# Attach Windows PE resource metadata (company/version/DpiAware) via go-winres.
winres:
	./scripts/gen-winres.sh $(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH)

install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/$(APP)

version:
	@echo "v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH) ($(GIT_COMMIT))"

clean:
	rm -rf $(BIN)