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
}


# Deployment - creates a new deployment each time
# Note: Terraform will create a new deployment on every apply
# You may want to use aws_api_gateway_stage instead for better control
resource "aws_api_gateway_deployment" "main" {
  rest_api_id = var.api_gateway_rest_api_id

  depends_on = [
    aws_api_gateway_integration.send_saved_strategy_summary_emails,
  ]

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
