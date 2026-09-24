BINARY := gke-context-switcher
PREFIX ?= /usr/local
BINDIR := $(DESTDIR)$(PREFIX)/bin

.PHONY: build install uninstall clean

build:
	go build -o $(BINARY) .

install: build
	install -Dm755 $(BINARY) $(BINDIR)/$(BINARY)

uninstall:
	rm -f $(BINDIR)/$(BINARY)

clean:
	rm -f $(BINARY)
