-include .env
export $(shell sed 's/=.*//' .env)

.PHONY: build

build:
	cd cmd/metadata && go build -v -o ../../build/ .

debug: build
	docker-compose up -d db
	cd build && POSTGRES_HOST=localhost ./metadata

run:
	docker-compose up -d --build

metadata:
	docker-compose up -d db
	cd cmd/metadata && go run .
