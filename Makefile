migration:
	migrate create -ext sql -dir migrations/ -seq $(name)

db-models:
	jet -dsn=postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable -path=./internal/db/models

migrate:
	migrate -path migrations -database postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable database up 2

deploy-be:
	aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 326651360928.dkr.ecr.us-east-1.amazonaws.com
	docker build -t factorbacktest .
	docker tag factorbacktest:latest 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest:latest
	docker push 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest:latest

deploy-fe:
	cd frontend;npm run build;
	aws s3 sync ./frontend/build s3://factorbacktest.net
	aws s3 sync ./frontend/build s3://www.factorbacktest.net