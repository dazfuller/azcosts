test:
	go test -v ./...

cover:
	go test -v ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

