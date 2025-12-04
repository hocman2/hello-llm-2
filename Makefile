hello: hello.go
	go build -ldflags "-w -s" hello.go

clear:
	rm hello
