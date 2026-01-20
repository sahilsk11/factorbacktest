#!/usr/bin/env python3
"""
API Code Generator
Generates Go code and Terraform from api/endpoints.yaml
"""

try:
    import yaml
except ImportError:
    print("Error: PyYAML is not installed.")
    print("Install it with: pip install PyYAML==6.0.1")
    print("Or if using virtualenv: tools/env/bin/pip install PyYAML==6.0.1")
    exit(1)

import os
import re
from pathlib import Path
from typing import Dict, List, Any, Optional

PROJECT_ROOT = Path(__file__).parent.parent
API_DIR = PROJECT_ROOT / "api"
TERRAFORM_DIR = PROJECT_ROOT / "terraform"
ENDPOINTS_YAML = API_DIR / "endpoints.yaml"


def to_camel_case(snake_str: str) -> str:
    """Convert snake_case to CamelCase"""
    components = snake_str.split('_')
    return ''.join(x.capitalize() for x in components)


def to_snake_case(camel_str: str) -> str:
    """Convert CamelCase to snake_case"""
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', camel_str)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()


def go_type_from_yaml_type(field_type: str) -> str:
    """Convert YAML type annotation to Go type"""
    if field_type.endswith('?'):
        base_type = field_type[:-1]
        go_type = go_type_from_yaml_type(base_type)
        return f"*{go_type}"
    
    type_map = {
        'string': 'string',
        'int': 'int',
        'float': 'float64',
        'bool': 'bool',
        'uuid': 'uuid.UUID',
    }
    return type_map.get(field_type, 'string')


def generate_request_struct(endpoint: Dict[str, Any]) -> Optional[str]:
    """Generate Go struct for request body"""
    request = endpoint.get('request')
    if not request or request == {}:
        return None
    
    handler_name = endpoint['handler']
    struct_name = f"{to_camel_case(handler_name)}Request"
    
    lines = [f"type {struct_name} struct {{"]
    for field_name, field_type in request.items():
        go_type = go_type_from_yaml_type(str(field_type))
        json_tag = to_snake_case(field_name)
        lines.append(f"\t{to_camel_case(field_name)} {go_type} `json:\"{json_tag}\"`")
    lines.append("}")
    return "\n".join(lines)


def generate_response_struct(endpoint: Dict[str, Any]) -> str:
    """Generate Go struct for response body"""
    handler_name = endpoint['handler']
    struct_name = f"{to_camel_case(handler_name)}Response"
    
    response = endpoint.get('response', {})
    if not response:
        return f"type {struct_name} map[string]interface{{}}"
    
    lines = [f"type {struct_name} struct {{"]
    for field_name, field_type in response.items():
        go_type = go_type_from_yaml_type(str(field_type))
        json_tag = to_snake_case(field_name)
        lines.append(f"\t{to_camel_case(field_name)} {go_type} `json:\"{json_tag}\"`")
    lines.append("}")
    return "\n".join(lines)


def generate_resolver_file(endpoint: Dict[str, Any]) -> str:
    """Generate resolver file content"""
    handler_name = endpoint['handler']
    file_name = to_snake_case(handler_name)
    
    request_struct = generate_request_struct(endpoint)
    response_struct = generate_response_struct(endpoint)
    
    has_request = request_struct is not None
    
    imports = [
        'package api',
        '',
        'import (',
        '\t"context"',
        '\t"factorbacktest/internal/domain"',
        '\t"factorbacktest/internal/logger"',
        '\t"fmt"',
        '',
        '\t"github.com/gin-gonic/gin"',
    ]
    
    if has_request:
        imports.insert(-1, '\t"encoding/json"')
    
    imports.append(')')
    imports.append('')
    
    content = '\n'.join(imports)
    content += '\n\n'
    
    if request_struct:
        content += request_struct + '\n\n'
    
    content += response_struct + '\n\n'
    
    # Handler function
    handler_func = f"""func (m ApiHandler) {handler_name}(c *gin.Context) {{
\tlg := logger.FromContext(c)
\tctx := c.Request.Context()

\t// Create performance profile (required by some services)
\tprofile, endProfile := domain.NewProfile()
\tdefer endProfile()
\tctx = context.WithValue(ctx, domain.ContextProfileKey, profile)

\t// Add logger to context
\tctx = context.WithValue(ctx, logger.ContextKey, lg)
"""
    
    if has_request:
        handler_func += f"""
\tvar requestBody {to_camel_case(handler_name)}Request
\tif err := c.ShouldBindJSON(&requestBody); err != nil {{
\t\treturnErrorJson(err, c)
\t\treturn
\t}}
"""
    
    handler_func += """
\t// TODO: Implement handler logic

\tlg.Info("handler completed successfully")
\tc.JSON(200, """ + f"{to_camel_case(handler_name)}Response" + """{
\t\t// TODO: Populate response
\t})
}
"""
    
    content += handler_func
    return content


def update_api_go(endpoints: List[Dict[str, Any]]) -> None:
    """Update api.go to register new endpoints"""
    api_go_path = API_DIR / "api.go"
    
    with open(api_go_path, 'r') as f:
        lines = f.readlines()
    
    # Find the route registration section
    # Look for the pattern: engine.GET/POST/PUT/DELETE(...)
    route_start_idx = None
    route_end_idx = None
    
    for i, line in enumerate(lines):
        stripped = line.strip()
        # Find first route registration
        if route_start_idx is None and stripped.startswith('engine.') and ('GET(' in stripped or 'POST(' in stripped or 'PUT(' in stripped or 'DELETE(' in stripped):
            route_start_idx = i
        # Find return statement after routes
        if route_start_idx is not None and 'return engine' in stripped:
            route_end_idx = i
            break
    
    if route_start_idx is None or route_end_idx is None:
        print("Warning: Could not find route registration section in api.go")
        print("  You may need to manually add routes to InitializeRouterEngine()")
        return
    
    # Extract existing routes (preserve comments and whitespace)
    existing_routes = []
    existing_paths = set()
    
    for i in range(route_start_idx, route_end_idx):
        line = lines[i]
        stripped = line.strip()
        if stripped.startswith('engine.') and ('GET(' in stripped or 'POST(' in stripped or 'PUT(' in stripped or 'DELETE(' in stripped):
            existing_routes.append(line.rstrip('\n'))
            # Extract path from line (e.g., engine.POST("/path", ...) -> /path
            import re
            match = re.search(r'["\']([^"\']+)["\']', stripped)
            if match:
                existing_paths.add(match.group(1))
    
    # Generate route registrations for new endpoints only
    new_routes = []
    for endpoint in endpoints:
        method = endpoint['method'].upper()
        path = endpoint['path']
        handler = endpoint['handler']
        
        # Skip if route already exists
        if path in existing_paths:
            print(f"  Skipping {path} (already registered)")
            continue
        
        new_routes.append(f'\tengine.{method}("{path}", m.{handler})')
    
    if not new_routes:
        print(f"No new routes to add to {api_go_path}")
        return
    
    # Insert new routes before the return statement
    # Preserve the last blank line before return if it exists
    insert_idx = route_end_idx
    if route_end_idx > 0 and lines[route_end_idx - 1].strip() == '':
        insert_idx = route_end_idx - 1
    
    # Build new file content
    new_lines = lines[:insert_idx] + new_routes + [''] + lines[route_end_idx:]
    
    with open(api_go_path, 'w') as f:
        f.writelines(new_lines)
    
    print(f"Updated {api_go_path} with {len(new_routes)} new route(s)")


def generate_resolver_files(endpoints: List[Dict[str, Any]]) -> None:
    """Generate resolver files for endpoints that don't exist"""
    for endpoint in endpoints:
        handler_name = endpoint['handler']
        file_name = to_snake_case(handler_name)
        resolver_path = API_DIR / f"{file_name}.resolver.go"
        
        if resolver_path.exists():
            print(f"Skipping {resolver_path} (already exists)")
            continue
        
        content = generate_resolver_file(endpoint)
        with open(resolver_path, 'w') as f:
            f.write(content)
        print(f"Generated {resolver_path}")


def generate_terraform(endpoints: List[Dict[str, Any]]) -> None:
    """Generate Terraform configuration for API Gateway"""
    TERRAFORM_DIR.mkdir(exist_ok=True)
    
    terraform_content = '''# Auto-generated API Gateway Terraform configuration
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

provider "aws" {
  region = var.aws_region
}

data "aws_lambda_function" "api" {
  function_name = var.lambda_function_name
}

data "aws_api_gateway_rest_api" "main" {
  id = var.api_gateway_rest_api_id
}

'''
    
    # Track all integrations for deployment dependency
    integration_resources = []
    
    # Generate resources for each endpoint
    for endpoint in endpoints:
        path = endpoint['path']
        method = endpoint['method'].upper()
        
        # Convert path to Terraform-safe resource name
        # e.g., /sendSavedStrategySummaryEmails -> send_saved_strategy_summary_emails
        resource_name = to_snake_case(path.lstrip('/').replace('/', '_'))
        
        # For simple paths (single segment), create resource directly
        # For nested paths, would need to create parent resources (not handling for now)
        path_parts = [p for p in path.split('/') if p]
        
        if len(path_parts) == 1:
            # Simple path - create resource under root
            terraform_content += f'''
# Endpoint: {path} ({method})
resource "aws_api_gateway_resource" "{resource_name}" {{
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = data.aws_api_gateway_rest_api.main.root_resource_id
  path_part   = "{path_parts[0]}"
}}

resource "aws_api_gateway_method" "{resource_name}" {{
  rest_api_id   = var.api_gateway_rest_api_id
  resource_id   = aws_api_gateway_resource.{resource_name}.id
  http_method   = "{method}"
  authorization = "NONE"
}}

resource "aws_api_gateway_integration" "{resource_name}" {{
  rest_api_id = var.api_gateway_rest_api_id
  resource_id = aws_api_gateway_resource.{resource_name}.id
  http_method = aws_api_gateway_method.{resource_name}.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = data.aws_lambda_function.api.invoke_arn
}}

'''
            integration_resources.append(f'aws_api_gateway_integration.{resource_name}')
        else:
            print(f"Warning: Nested paths not yet supported: {path}")
    
    # Generate deployment
    terraform_content += '''
# Deployment - creates a new deployment each time
# Note: Terraform will create a new deployment on every apply
# You may want to use aws_api_gateway_stage instead for better control
resource "aws_api_gateway_deployment" "main" {
  rest_api_id = var.api_gateway_rest_api_id
  stage_name  = "prod"

  depends_on = [
'''
    
    for integration in integration_resources:
        terraform_content += f'    {integration},\n'
    
    terraform_content += '''  ]

  lifecycle {
    create_before_destroy = true
  }
}

# Lambda permission to allow API Gateway to invoke
resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = data.aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${data.aws_api_gateway_rest_api.main.execution_arn}/*/*"
}

# Output the API Gateway URL
output "api_gateway_url" {
  value = "https://${data.aws_api_gateway_rest_api.main.id}.execute-api.${var.aws_region}.amazonaws.com/prod"
}
'''
    
    terraform_file = TERRAFORM_DIR / "api_gateway.tf"
    with open(terraform_file, 'w') as f:
        f.write(terraform_content)
    
    # Also create a variables.tf.example
    variables_example = '''# Copy this to terraform.tfvars and fill in your values
api_gateway_rest_api_id = "your-api-gateway-id-here"
lambda_function_name    = "fbTestArm"
aws_region              = "us-east-1"
'''
    
    variables_file = TERRAFORM_DIR / "terraform.tfvars.example"
    with open(variables_file, 'w') as f:
        f.write(variables_example)
    
    print(f"Generated {terraform_file}")
    print(f"Generated {variables_file}")
    print("\n  Next steps:")
    print("  1. Find your API Gateway ID:")
    print("     aws apigateway get-rest-apis --query 'items[*].[name,id]' --output table")
    print("  2. Copy terraform.tfvars.example to terraform.tfvars and fill in values")
    print("  3. Run: terraform -chdir=terraform init")
    print("  4. Run: terraform -chdir=terraform plan")
    print("  5. Run: terraform -chdir=terraform apply")


def main():
    """Main entry point"""
    if not ENDPOINTS_YAML.exists():
        print(f"Error: {ENDPOINTS_YAML} not found")
        return 1
    
    with open(ENDPOINTS_YAML, 'r') as f:
        config = yaml.safe_load(f)
    
    endpoints = config.get('endpoints', [])
    if not endpoints:
        print("No endpoints defined in endpoints.yaml")
        return 1
    
    print(f"Processing {len(endpoints)} endpoint(s)...")
    
    # Generate resolver files
    generate_resolver_files(endpoints)
    
    # Update api.go
    update_api_go(endpoints)
    
    # Generate Terraform
    generate_terraform(endpoints)
    
    print("\nDone! Next steps:")
    print("1. Review generated resolver files")
    print("2. Implement handler logic in resolver files")
    print("3. Review terraform/api_gateway.tf and update with your API Gateway ID")
    print("4. Run: terraform -chdir=terraform init && terraform -chdir=terraform plan")
    
    return 0


if __name__ == '__main__':
    exit(main())
