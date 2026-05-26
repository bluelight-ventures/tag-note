.PHONY: all build run stop clean release staging status dashboard

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

all: build

build:
	docker compose build

run:
	docker compose up --build -d

stop:
	docker compose down

clean:
	docker compose down
	docker image prune -f

release:
	./release/deploy.sh $(VERSION)

staging:
	./release/promote-staging.sh $(VERSION)

status:
	./release/status.sh

dashboard:
	./release/dashboard.sh
