build: 
	go build -ldflags "-w -s" hello.go

run: 
	go run hello.go

clear:
	rm hello
