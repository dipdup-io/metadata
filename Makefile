-include .env
export $(shell sed 's/=.*//' .env)

.PHONY: build

build:
	cd cmd/metadata && go build -v -o ../../dist/ .

debug: build
	docker-compose -f docker-compose.yml up -d db hasura
	cd dist && POSTGRES_HOST=localhost HASURA_HOST=localhost ./metadata -c ../build/dipdup.yml

up:
	docker-compose -f docker-compose.yml up -d --build

down:
	docker-compose -f docker-compose.yml down -v

metadata:
	cd cmd/metadata && go run . -c ../../build/dipdup.yml

lint:
	golangci-lint run

test:
	go test ./...

integration-test:
	docker-compose -f docker-compose.test.yml up -d
	until </dev/tcp/localhost/8080; do sleep 15; done
	cd cmd/metadata && INTEGRATION=true HASURA_HOST=127.0.0.1 HASURA_PORT=8080 bash -c 'go1.19 test -v -timeout=15s -run TestIntegration_HasuraMetadata' || true
	docker-compose -f docker-compose.test.yml down -v