run:
	go run ./cmd/orders/main.go

install:
	go install ./cmd/orders/main.go

install-oapi-codegen:
	go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

generate:
	go tool oapi-codegen -o ./internal/api/gen.go -config ./oapi-cfg.yaml ./api/openapi-spec/openapi.yaml

runCompose:
	docker-compose -f docker-compose.yml up --build