all: clean build 
build: 
	go build -o bin/hummingbard cmd/hummingbard/main.go
vendor: clean vendorbuild 
vendorbuild:
	go build -mod=vendor -o bin/hummingbard cmd/hummingbard/main.go
clean: 
	rm -f bin/hummingbard
