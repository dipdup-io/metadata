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