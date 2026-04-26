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
	mockgen -source=internal/repository/ses_email.repository.go -destination=internal/repository/mocks/mock_ses_email.repository.go
	mockgen -source=internal/repository/email_otp.repository.go -destination=internal/repository/mocks/mock_email_otp.repository.go

	# l2 services
	mockgen -source=internal/calculator/factor_expression.service.go -destination=internal/calculator/mocks/mock_factor_expression.service.go

	# services
	mockgen -source=internal/service/email.service.go -destination=internal/service/mocks/mock_email.service.go
	mockgen -source=internal/data/price.service.go -destination=internal/data/mocks/mock_price.service.go

	# apps
	mockgen -source=internal/app/strategy_summary.app.go -destination=internal/app/mocks/mock_strategy_summary.app.go


migration:
	migrate create -ext sql -dir migrations/ -seq $(name)

PYTHON ?= tools/env/bin/python

db-models:
	jet -dsn=postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable -path=./internal/db/models
	jet -dsn=postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable -schema=app_auth -path=./internal/db/models
	$(PYTHON) tools/db_model_helper.py

# Regenerates DB models against the live database and fails if the result
# differs from what's checked in. Used by CI to catch hand-edits to generated
# files or migrations that weren't accompanied by a model regeneration.
db-models-check: db-models
	@if ! git diff --quiet -- internal/db/models || \
	    [ -n "$$(git ls-files --others --exclude-standard -- internal/db/models)" ]; then \
		echo ""; \
		echo "ERROR: generated DB models are out of date."; \
		echo "Run 'make db-models' locally and commit the result."; \
		echo ""; \
		git status --short -- internal/db/models; \
		echo ""; \
		git --no-pager diff -- internal/db/models; \
		exit 1; \
	fi

migrate:
	$(PYTHON) tools/migrations.py up postgres

deploy-fe:
	cd frontend-v2;npm run build;
	aws s3 sync ./frontend-v2/dist s3://factorbacktest.net
	aws s3 sync ./frontend-v2/dist s3://www.factorbacktest.net
	aws s3 sync ./frontend-v2/dist s3://factor.trade
	aws s3 sync ./frontend-v2/dist s3://www.factor.trade
	rm -rf ./frontend-v2/dist;
	aws cloudfront create-invalidation --distribution-id E2LDUUB6BBDSV8 --paths "/*" --output text
	aws cloudfront create-invalidation --distribution-id E28M2984LB2P97 --paths "/*" --output text

deploy-fly:
	flyctl deploy --remote-only --build-arg commit_hash=$(shell git rev-parse --short HEAD)

deploy:
	make deploy-fly;
	make deploy-fe;

test:
	echo $(shell git rev-parse --short HEAD)

generate-api:
	tools/env/bin/python tools/generate_api.py