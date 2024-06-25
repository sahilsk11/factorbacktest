migration:
	migrate create -ext sql -dir migrations/ -seq $(name)

db-models:
	jet -dsn=postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable -path=./internal/db/models

migrate:
	migrate -path migrations -database postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable database up 2

deploy:
	docker build -t factorbacktest .
	docker tag factorbacktest:latest 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest:latest
	docker push 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest:latest