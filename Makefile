.PHONY: build

build:
	cd cmd/metadata && go build -v -o ../../build/ .

debug: build
	docker-compose up -d db
	cd build && POSTGRES_HOST=localhost ./metadata

run:
	docker-compose up -d --build