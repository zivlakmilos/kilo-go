all: run

run: build
	@./bin/kilo

build:
	@go build -o ./bin/kilo main.go
