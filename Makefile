BINARY   := skyfired
CLIENT   := skyfire-client
PREFIX   ?= /usr/local
ETC      ?= /etc/skyfire
BINROOT  := $(PREFIX)/bin
EMBDIR   := backend/internal/web/dist
BINDIR   := backend
CLIENTDIR := client

.PHONY: all web backend client client-windows client-darwin client-cli install uninstall run demo test clean

all: web backend

# Build the React frontend and stage it into the Go embed directory.
web:
	cd frontend && pnpm install && pnpm build
	mkdir -p $(EMBDIR)
	cp -r frontend/dist/index.html frontend/dist/assets $(EMBDIR)/

backend:
	cd $(BINDIR) && go build -trimpath -ldflags "-s -w" -o ../$(BINARY) ./cmd/skyfired

# Desktop client for the current platform. Windows/macOS use fyne (cgo); on
# Linux this builds the terminal client.
client:
	cd $(CLIENTDIR) && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o ../$(CLIENT) .

# Windows client (fyne tray + dialogs). Cross-compiling needs mingw-w64:
#   sudo apt-get install gcc-mingw-w64-x86-64
client-windows:
	cd $(CLIENTDIR) && CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
		go build -trimpath -ldflags "-s -w" -o ../$(CLIENT)-windows-amd64.exe .

# macOS client (fyne tray + dialogs). Needs cgo/Cocoa, so build it on macOS.
client-darwin:
	cd $(CLIENTDIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
		go build -trimpath -ldflags "-s -w" -o ../$(CLIENT)-darwin-arm64 .

# Linux client without the GUI (terminal fallback); no cgo, no GL headers.
client-cli:
	cd $(CLIENTDIR) && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ../$(CLIENT)-linux-amd64 .

install: all
	install -d $(DESTDIR)$(BINROOT) $(DESTDIR)$(ETC)
	install -m 0755 $(BINARY) $(DESTDIR)$(BINROOT)/skyfired
	ln -sf skyfired $(DESTDIR)$(BINROOT)/skyfire
	install -m 0644 -D deploy/skyfire.service $(DESTDIR)/lib/systemd/system/skyfire.service
	echo "Installed. Password is generated at first start and printed to the journal:"
	echo "  journalctl -u skyfire -n 20"

# run the daemon with the embedded UI (needs root for real tuning; use `make demo`)
run: all
	sudo ./$(BINARY) -config $(ETC)/config.json -addr :51821

# safe development mode: mock driver, sample data, no system changes.
demo: all
	./$(BINARY) -demo -addr :51821

test:
	cd $(BINDIR) && go vet ./... && go test ./...
	# CGO_ENABLED=0 keeps this portable; CI builds the cgo/fyne path separately.
	cd $(CLIENTDIR) && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go build ./...

clean:
	rm -f $(BINARY) $(CLIENT) $(CLIENT)-windows-amd64.exe $(CLIENT)-darwin-arm64
	rm -rf frontend/dist
	rm -rf $(EMBDIR)
