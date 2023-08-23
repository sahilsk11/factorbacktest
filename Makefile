migration:
	migrate create -ext sql -dir migrations/ -seq $(name)

db-models:
	jet -dsn=postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable -path=./internal/db/models

migrate:
	migrate -path migrations -database postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable database up 2
