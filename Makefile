.PHONY: build test vet fmt fmtcheck check clean

build:
	go build -o lt ./cmd/lt

test:
	go test -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmtcheck:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "needs gofmt:"; echo "$$out"; exit 1; fi

check: fmtcheck vet test build

clean:
	rm -f lt
