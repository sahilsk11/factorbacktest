mocks:
	mockgen -source=internal/repository/adj_price.repository.go -destination=internal/repository/mocks/mock_adj_price.repository.go

migration:
	migrate create -ext sql -dir migrations/ -seq $(name)

db-models:
	jet -dsn=postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable -path=./internal/db/models

migrate:
	tools/env/bin/python tools/migrations.py up postgres
	tools/env/bin/python tools/migrations.py up postgres_test

deploy-fe:
	cd frontend;npm run build;
	aws s3 sync ./frontend/build s3://factorbacktest.net
	aws s3 sync ./frontend/build s3://www.factorbacktest.net
	rm -rf ./frontend/build;
	aws cloudfront create-invalidation --distribution-id E2LDUUB6BBDSV8 --paths "/*"

deploy-lambda:
	aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 326651360928.dkr.ecr.us-east-1.amazonaws.com
	docker build --platform linux/arm64 -t factorbacktest_lambda -f Dockerfile.lambda --build-arg commit_hash=$(shell git rev-parse --short HEAD) --build-arg GIN_MODE=release .
	docker tag factorbacktest_lambda:latest 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest_lambda:latest
	docker push 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest_lambda:latest
	aws lambda update-function-code --region us-east-1 --function-name fbTestArm --image-uri 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest_lambda:latest
	aws lambda update-function-configuration --region us-east-1 --function-name fbTestArm --environment "Variables={commit_hash=$(shell git rev-parse --short HEAD),GIN_MODE=release}"

update-lambda-config:
	aws lambda update-function-configuration --region us-east-1 --function-name fbTestArm --environment "Variables={commit_hash=$(shell git rev-parse --short HEAD),GIN_MODE=release}"

deploy:
	make deploy-lambda;
	make deploy-fe;

test:
	echo $(shell git rev-parse --short HEAD)