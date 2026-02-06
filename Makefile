PREFIX  ?= $(HOME)/.local
BINDIR  ?= $(PREFIX)/bin
BINARY  := just-stream
GOFLAGS ?=

.PHONY: build install uninstall clean windows

build:
	go build $(GOFLAGS) -o $(BINARY) .

install: build
	install -d $(BINDIR)
	install -m 755 $(BINARY) $(BINDIR)/$(BINARY)

uninstall:
	rm -f $(BINDIR)/$(BINARY)

windows:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o $(BINARY).exe .

clean:
	rm -f $(BINARY) $(BINARY).exe
