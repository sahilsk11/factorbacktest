mocks:
	# repositories
	rm internal/repository/mocks/*
	mockgen -source=internal/repository/interest_rate.repository.go -destination=internal/repository/mocks/mock_interest_rate.repository.go
	mockgen -source=internal/repository/adj_price.repository.go -destination=internal/repository/mocks/mock_adj_price.repository.go
	mockgen -source=internal/repository/alpaca.repository.go -destination=internal/repository/mocks/mock_alpaca.repository.go
	mockgen -source=internal/repository/investment.repository.go -destination=internal/repository/mocks/mock_investment.repository.go
	mockgen -source=internal/repository/strategy.repository.go -destination=internal/repository/mocks/mock_strategy.repository.go
	mockgen -source=internal/repository/investment_holdings.repository.go -destination=internal/repository/mocks/mock_investment_holdings.repository.go
	mockgen -source=internal/repository/asset_universe.repository.go -destination=internal/repository/mocks/mock_asset_universe.repository.go
	mockgen -source=internal/repository/ticker.repository.go -destination=internal/repository/mocks/mock_ticker.repository.go
	mockgen -source=internal/repository/investment_rebalance.repository.go -destination=internal/repository/mocks/mock_investment_rebalance.repository.go
	mockgen -source=internal/repository/investment_trade.repository.go -destination=internal/repository/mocks/mock_investment_trade.repository.go
	mockgen -source=internal/repository/investment_holdings_version.repository.go -destination=internal/repository/mocks/mock_investment_holdings_version.repository.go
	mockgen -source=internal/repository/excess_trade_volume.repository.go -destination=internal/repository/mocks/mock_excess_trade_volume.repository.go
	mockgen -source=internal/repository/trade_order.repository.go -destination=internal/repository/mocks/mock_trade_order.repository.go
	mockgen -source=internal/repository/rebalancer_run.repository.go -destination=internal/repository/mocks/mock_rebalancer_run.repository.go

	# l2 services
	mockgen -source=internal/calculator/factor_expression.service.go -destination=internal/calculator/mocks/mock_factor_expression.service.go


migration:
	migrate create -ext sql -dir migrations/ -seq $(name)

db-models:
	jet -dsn=postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable -path=./internal/db/models
	tools/env/bin/python tools/db_model_helper.py

migrate:
	tools/env/bin/python tools/migrations.py up postgres
	tools/env/bin/python tools/migrations.py up postgres_test

deploy-fe:
	cd frontend;npm run build;
	aws s3 sync ./frontend/build s3://factorbacktest.net
	aws s3 sync ./frontend/build s3://www.factorbacktest.net
	aws s3 sync ./frontend/build s3://factor.trade
	aws s3 sync ./frontend/build s3://www.factor.trade
	rm -rf ./frontend/build;
	aws cloudfront create-invalidation --distribution-id E2LDUUB6BBDSV8 --paths "/*" --output text
	aws cloudfront create-invalidation --distribution-id E28M2984LB2P97 --paths "/*" --output text

deploy-lambda:
	aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 326651360928.dkr.ecr.us-east-1.amazonaws.com
	docker build --platform linux/arm64 -t factorbacktest_lambda -f Dockerfile.lambda --build-arg commit_hash=$(shell git rev-parse --short HEAD) --build-arg GIN_MODE=release .
	docker tag factorbacktest_lambda:latest 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest_lambda:latest
	docker push 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest_lambda:latest
	aws lambda update-function-code --region us-east-1 --function-name fbTestArm --image-uri 326651360928.dkr.ecr.us-east-1.amazonaws.com/factorbacktest_lambda:latest --output text
	aws lambda wait function-updated --region us-east-1 --function-name fbTestArm
	aws lambda update-function-configuration --region us-east-1 --function-name fbTestArm --environment "Variables={commit_hash=$(shell git rev-parse --short HEAD),GIN_MODE=release}" --output text

update-lambda-config:
	aws lambda update-function-configuration --region us-east-1 --function-name fbTestArm --environment "Variables={commit_hash=$(shell git rev-parse --short HEAD),GIN_MODE=release}" --output text

deploy:
	make deploy-lambda;
	make deploy-fe;

test:
	echo $(shell git rev-parse --short HEAD)