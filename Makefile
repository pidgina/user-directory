DB   ?= users.db
ADDR ?= :8080
LIMIT ?= 500

.PHONY: run serve test build clean

run:
	go run . -db $(DB) -limit $(LIMIT)

serve:
	go run ./cmd/server -db $(DB) -addr $(ADDR)

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/loader .
	go build -o bin/server ./cmd/server

clean:
	rm -rf bin $(DB) $(DB)-wal $(DB)-shm
