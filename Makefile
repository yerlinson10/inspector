.PHONY: run build clean

run:
	go run main.go

build:
	go build -o inspector.exe main.go

clean:
	del /f inspector.exe inspector.db 2>nul || true
