# Auto-generated API Gateway Terraform configuration
# Run: terraform init && terraform plan && terraform apply
#
# IMPORTANT: Update api_gateway_rest_api_id with your existing API Gateway ID
# You can find it in AWS Console or run: aws apigateway get-rest-apis --query 'items[?name==`YOUR_API_NAME`].id' --output text

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "api_gateway_rest_api_id" {
  description = "API Gateway REST API ID (find in AWS Console)"
  type        = string
  # TODO: Replace with your actual API Gateway ID
  # You can also set via: terraform apply -var="api_gateway_rest_api_id=YOUR_ID"
}

variable "lambda_function_name" {
  description = "Lambda function name"
  type        = string
  default     = "fbTestArm"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "stage_name" {
  description = "API Gateway stage name"
  type        = string
  default     = "prod"
}

provider "aws" {
  region = var.aws_region
}

data "aws_caller_identity" "current" {}

data "aws_lambda_function" "api" {
  function_name = var.lambda_function_name
}

# Get root resource ID (path "/")
data "aws_api_gateway_resource" "root" {
  rest_api_id = var.api_gateway_rest_api_id
  path        = "/"
}

locals {
  # Root resource ID from data source
  root_resource_id = data.aws_api_gateway_resource.root.id
  
  # Execution ARN format: arn:aws:execute-api:region:account-id:api-id/*/*
  execution_arn = "arn:aws:execute-api:${var.aws_region}:${data.aws_caller_identity.current.account_id}:${var.api_gateway_rest_api_id}/*/*"
}


# Endpoint: /backtest (POST)
resource "aws_api_gateway_resource" "backtest" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "backtest"
}

resource "aws_api_gateway_method" "backtest" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.backtest.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "backtest" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.backtest.id
  http_method = aws_api_gateway_method.backtest.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 300
}


# Endpoint: /benchmark (POST)
resource "aws_api_gateway_resource" "benchmark" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "benchmark"
}

resource "aws_api_gateway_method" "benchmark" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.benchmark.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "benchmark" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.benchmark.id
  http_method = aws_api_gateway_method.benchmark.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /contact (POST)
resource "aws_api_gateway_resource" "contact" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "contact"
}

resource "aws_api_gateway_method" "contact" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.contact.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "contact" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.contact.id
  http_method = aws_api_gateway_method.contact.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /constructFactorEquation (POST)
resource "aws_api_gateway_resource" "construct_factor_equation" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "constructFactorEquation"
}

resource "aws_api_gateway_method" "construct_factor_equation" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.construct_factor_equation.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "construct_factor_equation" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.construct_factor_equation.id
  http_method = aws_api_gateway_method.construct_factor_equation.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 60
}


# Endpoint: /usageStats (GET)
resource "aws_api_gateway_resource" "usage_stats" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "usageStats"
}

resource "aws_api_gateway_method" "usage_stats" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.usage_stats.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "usage_stats" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.usage_stats.id
  http_method = aws_api_gateway_method.usage_stats.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /assetUniverses (GET)
resource "aws_api_gateway_resource" "asset_universes" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "assetUniverses"
}

resource "aws_api_gateway_method" "asset_universes" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.asset_universes.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "asset_universes" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.asset_universes.id
  http_method = aws_api_gateway_method.asset_universes.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /backtestBondPortfolio (POST)
resource "aws_api_gateway_resource" "backtest_bond_portfolio" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "backtestBondPortfolio"
}

resource "aws_api_gateway_method" "backtest_bond_portfolio" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.backtest_bond_portfolio.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "backtest_bond_portfolio" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.backtest_bond_portfolio.id
  http_method = aws_api_gateway_method.backtest_bond_portfolio.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 120
}


# Endpoint: /updatePrices (POST)
resource "aws_api_gateway_resource" "update_prices" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "updatePrices"
}

resource "aws_api_gateway_method" "update_prices" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.update_prices.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "update_prices" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.update_prices.id
  http_method = aws_api_gateway_method.update_prices.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 300
}


# Endpoint: /addAssetsToUniverse (POST)
resource "aws_api_gateway_resource" "add_assets_to_universe" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "addAssetsToUniverse"
}

resource "aws_api_gateway_method" "add_assets_to_universe" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.add_assets_to_universe.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "add_assets_to_universe" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.add_assets_to_universe.id
  http_method = aws_api_gateway_method.add_assets_to_universe.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 300
}


# Endpoint: /bookmarkStrategy (POST)
resource "aws_api_gateway_resource" "bookmark_strategy" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "bookmarkStrategy"
}

resource "aws_api_gateway_method" "bookmark_strategy" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.bookmark_strategy.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "bookmark_strategy" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.bookmark_strategy.id
  http_method = aws_api_gateway_method.bookmark_strategy.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /isStrategyBookmarked (POST)
resource "aws_api_gateway_resource" "is_strategy_bookmarked" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "isStrategyBookmarked"
}

resource "aws_api_gateway_method" "is_strategy_bookmarked" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.is_strategy_bookmarked.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "is_strategy_bookmarked" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.is_strategy_bookmarked.id
  http_method = aws_api_gateway_method.is_strategy_bookmarked.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /savedStrategies (GET)
resource "aws_api_gateway_resource" "saved_strategies" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "savedStrategies"
}

resource "aws_api_gateway_method" "saved_strategies" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.saved_strategies.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "saved_strategies" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.saved_strategies.id
  http_method = aws_api_gateway_method.saved_strategies.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /investInStrategy (POST)
resource "aws_api_gateway_resource" "invest_in_strategy" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "investInStrategy"
}

resource "aws_api_gateway_method" "invest_in_strategy" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.invest_in_strategy.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "invest_in_strategy" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.invest_in_strategy.id
  http_method = aws_api_gateway_method.invest_in_strategy.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 60
}


# Endpoint: /activeInvestments (GET)
resource "aws_api_gateway_resource" "active_investments" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "activeInvestments"
}

resource "aws_api_gateway_method" "active_investments" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.active_investments.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "active_investments" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.active_investments.id
  http_method = aws_api_gateway_method.active_investments.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 60
}


# Endpoint: /publishedStrategies (GET)
resource "aws_api_gateway_resource" "published_strategies" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "publishedStrategies"
}

resource "aws_api_gateway_method" "published_strategies" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.published_strategies.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "published_strategies" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.published_strategies.id
  http_method = aws_api_gateway_method.published_strategies.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 60
}


# Endpoint: /rebalance (POST)
resource "aws_api_gateway_resource" "rebalance" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "rebalance"
}

resource "aws_api_gateway_method" "rebalance" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.rebalance.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "rebalance" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.rebalance.id
  http_method = aws_api_gateway_method.rebalance.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 300
}


# Endpoint: /updateOrders (POST)
resource "aws_api_gateway_resource" "update_orders" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "updateOrders"
}

resource "aws_api_gateway_method" "update_orders" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.update_orders.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "update_orders" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.update_orders.id
  http_method = aws_api_gateway_method.update_orders.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 30
}


# Endpoint: /sendSavedStrategySummaryEmails (POST)
resource "aws_api_gateway_resource" "send_saved_strategy_summary_emails" {
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
  path_part   = "sendSavedStrategySummaryEmails"
}

resource "aws_api_gateway_method" "send_saved_strategy_summary_emails" {
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.send_saved_strategy_summary_emails.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "send_saved_strategy_summary_emails" {
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.send_saved_strategy_summary_emails.id
  http_method = aws_api_gateway_method.send_saved_strategy_summary_emails.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
  timeout = 120
}


# Deployment - creates a new deployment each time
# Note: Terraform will create a new deployment on every apply
# You may want to use aws_api_gateway_stage instead for better control
resource "aws_api_gateway_deployment" "main" {
  rest_api_id = var.api_gateway_rest_api_id

  depends_on = [
    aws_api_gateway_integration.backtest,
    aws_api_gateway_integration.benchmark,
    aws_api_gateway_integration.contact,
    aws_api_gateway_integration.construct_factor_equation,
    aws_api_gateway_integration.usage_stats,
    aws_api_gateway_integration.asset_universes,
    aws_api_gateway_integration.backtest_bond_portfolio,
    aws_api_gateway_integration.update_prices,
    aws_api_gateway_integration.add_assets_to_universe,
    aws_api_gateway_integration.bookmark_strategy,
    aws_api_gateway_integration.is_strategy_bookmarked,
    aws_api_gateway_integration.saved_strategies,
    aws_api_gateway_integration.invest_in_strategy,
    aws_api_gateway_integration.active_investments,
    aws_api_gateway_integration.published_strategies,
    aws_api_gateway_integration.rebalance,
    aws_api_gateway_integration.update_orders,
    aws_api_gateway_integration.send_saved_strategy_summary_emails,
  ]

  # Force a new deployment when the API surface changes.
  # Without this, Terraform can add resources/methods/integrations without
  # creating a new deployment, leaving the stage pointing at an older snapshot.
  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.backtest.id,
      aws_api_gateway_method.backtest.id,
      aws_api_gateway_integration.backtest.id,
      aws_api_gateway_resource.benchmark.id,
      aws_api_gateway_method.benchmark.id,
      aws_api_gateway_integration.benchmark.id,
      aws_api_gateway_resource.contact.id,
      aws_api_gateway_method.contact.id,
      aws_api_gateway_integration.contact.id,
      aws_api_gateway_resource.construct_factor_equation.id,
      aws_api_gateway_method.construct_factor_equation.id,
      aws_api_gateway_integration.construct_factor_equation.id,
      aws_api_gateway_resource.usage_stats.id,
      aws_api_gateway_method.usage_stats.id,
      aws_api_gateway_integration.usage_stats.id,
      aws_api_gateway_resource.asset_universes.id,
      aws_api_gateway_method.asset_universes.id,
      aws_api_gateway_integration.asset_universes.id,
      aws_api_gateway_resource.backtest_bond_portfolio.id,
      aws_api_gateway_method.backtest_bond_portfolio.id,
      aws_api_gateway_integration.backtest_bond_portfolio.id,
      aws_api_gateway_resource.update_prices.id,
      aws_api_gateway_method.update_prices.id,
      aws_api_gateway_integration.update_prices.id,
      aws_api_gateway_resource.add_assets_to_universe.id,
      aws_api_gateway_method.add_assets_to_universe.id,
      aws_api_gateway_integration.add_assets_to_universe.id,
      aws_api_gateway_resource.bookmark_strategy.id,
      aws_api_gateway_method.bookmark_strategy.id,
      aws_api_gateway_integration.bookmark_strategy.id,
      aws_api_gateway_resource.is_strategy_bookmarked.id,
      aws_api_gateway_method.is_strategy_bookmarked.id,
      aws_api_gateway_integration.is_strategy_bookmarked.id,
      aws_api_gateway_resource.saved_strategies.id,
      aws_api_gateway_method.saved_strategies.id,
      aws_api_gateway_integration.saved_strategies.id,
      aws_api_gateway_resource.invest_in_strategy.id,
      aws_api_gateway_method.invest_in_strategy.id,
      aws_api_gateway_integration.invest_in_strategy.id,
      aws_api_gateway_resource.active_investments.id,
      aws_api_gateway_method.active_investments.id,
      aws_api_gateway_integration.active_investments.id,
      aws_api_gateway_resource.published_strategies.id,
      aws_api_gateway_method.published_strategies.id,
      aws_api_gateway_integration.published_strategies.id,
      aws_api_gateway_resource.rebalance.id,
      aws_api_gateway_method.rebalance.id,
      aws_api_gateway_integration.rebalance.id,
      aws_api_gateway_resource.update_orders.id,
      aws_api_gateway_method.update_orders.id,
      aws_api_gateway_integration.update_orders.id,
      aws_api_gateway_resource.send_saved_strategy_summary_emails.id,
      aws_api_gateway_method.send_saved_strategy_summary_emails.id,
      aws_api_gateway_integration.send_saved_strategy_summary_emails.id,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Stage (preferred over aws_api_gateway_deployment.stage_name in newer providers)
resource "aws_api_gateway_stage" "main" {
  rest_api_id   = var.api_gateway_rest_api_id
  deployment_id = aws_api_gateway_deployment.main.id
  stage_name    = var.stage_name
}

# Lambda permission to allow API Gateway to invoke
resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = data.aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = local.execution_arn
}

# Output the API Gateway URL
output "api_gateway_url" {
  value = "https://${var.api_gateway_rest_api_id}.execute-api.${var.aws_region}.amazonaws.com/${var.stage_name}"
}
