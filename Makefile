CONFIG_DIR := $(HOME)/.config/gojira
INSTALL_DIR := $(HOME)/go/bin

.PHONY: install

install:
	go build -o bin/gojira
	mkdir -p $(CONFIG_DIR)
	cp bin/gojira $(INSTALL_DIR)/gojira
	cp templates.yaml $(CONFIG_DIR)/templates.yaml
	@echo "Done: gojira -> $(INSTALL_DIR)/gojira, templates.yaml -> $(CONFIG_DIR)/templates.yaml"
