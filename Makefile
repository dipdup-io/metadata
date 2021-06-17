-include .env
export $(shell sed 's/=.*//' .env)

CONFIG?=../../build/dipdup.yml

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
	cd cmd/metadata && go run . -c $(CONFIG)

elastic:
	docker-compose -f docker-compose.elastic.yml up -d elastic
	cd cmd/metadata && go run . -c ../../build/elastic.yml