APP_NAME := apiflow
OUT_DIR := ./bin

.PHONY: run build build-windows build-linux clean

run:
	@go run .

build:
	@mkdir -p $(OUT_DIR)
	@go build -o $(OUT_DIR)/$(APP_NAME) .

build-windows:
	@mkdir -p $(OUT_DIR)
	@GOOS=windows GOARCH=amd64 go build -o $(OUT_DIR)/$(APP_NAME).exe .

build-linux:
	@mkdir -p $(OUT_DIR)
	@GOOS=linux GOARCH=amd64 go build -o $(OUT_DIR)/$(APP_NAME)-linux .

build-all: build-windows build-linux

clean:
	@rm -rf $(OUT_DIR)

run-binary: build
	@$(OUT_DIR)/$(APP_NAME)
