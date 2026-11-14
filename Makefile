.PHONY: fmt-check vet lint vuln test check build up down logs backup scan drill
STATICCHECK = honnef.co/go/tools/cmd/staticcheck@2025.1.1
VERSION ?= dev
SCAN_EXIT ?= 1

# Same checks the CI pipeline runs.
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "unformatted files:"; gofmt -l .; exit 1)
vet:
	go vet ./...
lint:
	go run $(STATICCHECK) ./...
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
test:
	go test -race -count=1 ./...
check: fmt-check vet lint test

build:
	docker build --build-arg VERSION=$(VERSION) -t school-api:$(VERSION) .
up:
	./scripts/deploy.sh $(VERSION)
down:
	docker compose down
logs:
	docker compose logs -f --tail 50
backup:
	./scripts/backup.sh
scan: build
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.58.1 image --severity HIGH,CRITICAL --exit-code $(SCAN_EXIT) school-api:$(VERSION)
drill:
	./scripts/drill.sh
