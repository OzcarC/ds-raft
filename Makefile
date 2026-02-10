.PHONY: all build client server clean tidy

all: build

build: client server

client:
	cd client && go build -o ./../client.exe

server: 
	cd server && go build -o ./../server.exe

tidy:
	cd server && go mod tidy
	cd client && go mod tidy

clean:
	rm -f ./client.exe ./server.exe