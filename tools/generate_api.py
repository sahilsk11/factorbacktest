#!/usr/bin/env python3
"""
API Code Generator
Generates Go code and Terraform from api/openapi.yaml (OpenAPI 3.x format)
"""

try:
    import yaml
except ImportError:
    print("Error: PyYAML is not installed.")
    print("Install it with: pip install PyYAML")
    exit(1)

import os
import re
import json
import hashlib
from pathlib import Path
from typing import Dict, List, Any, Optional, Set

PROJECT_ROOT = Path(__file__).parent.parent
API_DIR = PROJECT_ROOT / "api"
TERRAFORM_DIR = PROJECT_ROOT / "terraform"
DOCS_DIR = PROJECT_ROOT / "docs"
OPENAPI_YAML = API_DIR / "openapi.yaml"


def to_camel_case(snake_str: str) -> str:
    """Convert snake_case to CamelCase, preserving case of non-first letters"""
    if "_" not in snake_str:
        # Already camelCase - just uppercase the first letter
        return snake_str[:1].upper() + snake_str[1:] if snake_str else snake_str
    components = snake_str.split('_')
    return ''.join(x[:1].upper() + x[1:] if x else x for x in components)


def to_exported_ident(name: str) -> str:
    """
    Convert a handler/type name into an exported Go identifier.
    - If it's snake_case, use CamelCase.
    - If it's already lowerCamel/camel, just upper-case the first letter.
    """
    if "_" in name:
        cc = to_camel_case(name)
        return cc[:1].upper() + cc[1:] if cc else cc
    return name[:1].upper() + name[1:] if name else name


def to_snake_case(camel_str: str) -> str:
    """Convert CamelCase to snake_case"""
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', camel_str)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()


def to_lower_camel_case(camel_str: str) -> str:
    """Convert CamelCase to lowerCamelCase (first letter lowercase)"""
    if not camel_str:
        return camel_str
    return camel_str[0].lower() + camel_str[1:] if len(camel_str) > 1 else camel_str.lower()


def resolve_ref(spec: Dict[str, Any], ref: str) -> Dict[str, Any]:
    """Resolve a $ref reference within the OpenAPI spec"""
    if not ref.startswith('#/'):
        return {}
    
    parts = ref.lstrip('#/').split('/')
    current = spec
    for part in parts:
        if part not in current:
            return {}
        current = current[part]
    return current


def getSchema_from_spec(spec: Dict[str, Any], schema_or_ref: Any) -> Dict[str, Any]:
    """Get schema dict, resolving $ref if needed"""
    if isinstance(schema_or_ref, dict):
        if '$ref' in schema_or_ref:
            return resolve_ref(spec, schema_or_ref['$ref'])
        return schema_or_ref
    return {}


def go_type_from_openapi_type(spec: Dict[str, Any], schema: Dict[str, Any], seen_refs: Set[str] = None) -> tuple[str, bool]:
    """
    Convert OpenAPI schema to Go type.
    Returns (go_type, needs_import) tuple.
    """
    if seen_refs is None:
        seen_refs = set()
    
    schema = getSchema_from_spec(spec, schema)
    if not schema:
        return 'interface{}', False
    
    # Handle array type
    if schema.get('type') == 'array':
        items = schema.get('items')
        if items:
            item_type, needs_import = go_type_from_openapi_type(spec, items, seen_refs)
            return f'[]{item_type}', needs_import
        return '[]interface{}', False
    
    # Handle object type with properties
    if schema.get('type') == 'object':
        if 'properties' in schema:
            # Generate inline struct - caller should handle naming
            return None, False  # Will be handled specially
        if 'additionalProperties' in schema:
            # Map type
            value_schema = schema.get('additionalProperties')
            if isinstance(value_schema, dict) and '$ref' in value_schema:
                value_type, needs_import = go_type_from_openapi_type(spec, value_schema, seen_refs)
                return f'map[string]{value_type}', needs_import
            elif isinstance(value_schema, dict):
                value_type, needs_import = go_type_from_openapi_type(spec, value_schema, seen_refs)
                return f'map[string]{value_type}', needs_import
            return 'map[string]interface{}', False
    
    # Handle $ref
    if '$ref' in schema:
        ref = schema['$ref']
        seen_refs.add(ref)
        resolved = resolve_ref(spec, ref)
        type_name = ref.split('/')[-1]
        # Check for uuid format
        if resolved.get('format') == 'uuid':
            return 'uuid.UUID', True
        if resolved.get('type') == 'object' and 'properties' in resolved:
            # Return the ref name, will be handled by caller
            return type_name, False
        return type_name, False
    
    # Handle base types
    field_type = schema.get('type')
    field_format = schema.get('format')
    
    # Check nullable - return pointer type
    nullable = schema.get('nullable', False)
    
    # UUID format
    if field_type == 'string' and field_format == 'uuid':
        return 'uuid.UUID', True
    
    # date-time format
    if field_type == 'string' and field_format == 'date-time':
        return 'string', False  # Use string for time.Time in models
    
    # date format
    if field_type == 'string' and field_format == 'date':
        return 'string', False
    
    type_map = {
        'string': 'string',
        'integer': 'int',
        'number': 'float64',
        'boolean': 'bool',
    }
    
    go_type = type_map.get(field_type, 'string')
    
    if nullable:
        return f'*{go_type}', False
    
    return go_type, False


def generate_struct_fields(spec: Dict[str, Any], schema: Dict[str, Any], struct_name: str, seen_refs: Set[str] = None) -> List[str]:
    """Generate Go struct fields from OpenAPI schema properties"""
    if seen_refs is None:
        seen_refs = set()
    
    schema = getSchema_from_spec(spec, schema)
    if not schema or 'properties' not in schema:
        return []
    
    lines = []
    for prop_name, prop_schema in schema.get('properties', {}).items():
        prop_schema = getSchema_from_spec(spec, prop_schema)
        
        # Check if it's an array with items.$ref
        if prop_schema.get('type') == 'array':
            items = prop_schema.get('items', {})
            if '$ref' in items:
                ref = items['$ref']
                ref_name = ref.split('/')[-1]
                resolved = resolve_ref(spec, ref)
                # Check if items is a simple type
                if resolved.get('type') == 'object' and 'properties' in resolved:
                    prop_schema = items  # Will use ref directly
                    go_type = ref_name
                else:
                    go_type = f'[]{ref_name}'
            elif 'type' in items:
                item_type, _ = go_type_from_openapi_type(spec, items, seen_refs)
                go_type = f'[]{item_type}'
            else:
                go_type = '[]interface{}'
        elif '$ref' in prop_schema:
            go_type = prop_schema['$ref'].split('/')[-1]
        else:
            go_type, _ = go_type_from_openapi_type(spec, prop_schema, seen_refs)
        
        json_tag = to_lower_camel_case(prop_name)
        field_name = to_camel_case(prop_name)
        lines.append(f'\t{field_name} {go_type} `json:"{json_tag}"`')
    
    return lines


def collect_schemas(spec: Dict[str, Any]) -> Dict[str, Dict[str, Any]]:
    """Collect all schemas from components/schemas"""
    schemas = {}
    components = spec.get('components', {})
    schema_dict = components.get('schemas', {})
    
    for name, schema in schema_dict.items():
        schemas[name] = schema
    
    return schemas


def generate_inline_struct_fields(spec: Dict[str, Any], schema: Dict[str, Any], seen_refs: Set[str]) -> str:
    """Generate inline struct fields as a semicolon-separated string for embedding"""
    if seen_refs is None:
        seen_refs = set()
    
    schema = getSchema_from_spec(spec, schema)
    if not schema or 'properties' not in schema:
        return ''
    
    fields = []
    for prop_name, prop_schema in schema.get('properties', {}).items():
        prop_schema = getSchema_from_spec(spec, prop_schema)
        
        if prop_schema.get('type') == 'array':
            items = prop_schema.get('items', {})
            if '$ref' in items:
                ref_name = items['$ref'].split('/')[-1]
                go_type = f"[]{ref_name}"
            elif 'type' in items:
                item_type, _ = go_type_from_openapi_type(spec, items, seen_refs)
                go_type = f"[]{item_type}"
            else:
                go_type = "[]interface{}"
        elif '$ref' in prop_schema:
            go_type = prop_schema['$ref'].split('/')[-1]
        elif prop_schema.get('additionalProperties'):
            additional = prop_schema.get('additionalProperties')
            if '$ref' in additional:
                value_type = additional['$ref'].split('/')[-1]
            else:
                value_type, _ = go_type_from_openapi_type(spec, additional, seen_refs)
                if value_type is None:
                    value_type = 'interface{}'
            go_type = f"map[string]{value_type}"
        elif prop_schema.get('type') == 'object' and 'properties' in prop_schema:
            # Recursively embed nested inline structs
            nested_fields = generate_inline_struct_fields(spec, prop_schema, seen_refs)
            go_type = f"struct {{ {nested_fields} }}"
        else:
            go_type, _ = go_type_from_openapi_type(spec, prop_schema, seen_refs)
            if go_type is None:
                go_type = 'interface{}'
        
        json_tag = to_lower_camel_case(prop_name)
        field_name = to_camel_case(prop_name)
        fields.append(f'{field_name} {go_type} `json:"{json_tag}"`')
    
    return '; '.join(fields)


def generate_models_file(endpoints: List[Dict[str, Any]], spec: Dict[str, Any]) -> None:
    """Generate shared request/response types into api/models (apimodels package)."""
    models_dir = API_DIR / "models"
    models_dir.mkdir(exist_ok=True)
    models_path = models_dir / "generated.go"
    
    schemas = collect_schemas(spec)
    
    lines: List[str] = []
    lines.append("package apimodels")
    lines.append("")
    lines.append("// Code generated by tools/generate_api.py. DO NOT EDIT.")
    lines.append("// Source: api/openapi.yaml")
    lines.append("")
    
    # Collect imports
    needs_uuid = False
    generated_structs: Set[str] = set()
    
    # Pre-process schemas to find uuid usage
    for schema_name, schema in schemas.items():
        schema = getSchema_from_spec(spec, schema)
        if not schema:
            continue
        
        if 'properties' in schema:
            for prop_name, prop_schema in schema.get('properties', {}).items():
                prop_schema = getSchema_from_spec(spec, prop_schema)
                
                if prop_schema.get('format') == 'uuid':
                    needs_uuid = True
                
                if prop_schema.get('type') == 'array':
                    items = prop_schema.get('items', {})
                    if '$ref' in items:
                        ref_name = items['$ref'].split('/')[-1]
                        item_schema = resolve_ref(spec, items['$ref'])
                        if item_schema.get('format') == 'uuid' or (item_schema.get('properties', {}) and any(
                            p.get('format') == 'uuid' for p in item_schema.get('properties', {}).values()
                        )):
                            needs_uuid = True
    
    if needs_uuid:
        lines.append("import (")
        lines.append('\t"github.com/google/uuid"')
        lines.append(")")
        lines.append("")
    
    # Generate structs for schemas
    for schema_name, schema in schemas.items():
        schema = getSchema_from_spec(spec, schema)
        if not schema:
            continue
        
        if 'properties' in schema:
            struct_name = schema_name
            if struct_name.endswith('Request'):
                struct_name = struct_name[:-7] + "Request"
            elif struct_name.endswith('Response'):
                struct_name = struct_name[:-8] + "Response"
            
            lines.append(f"type {struct_name} struct {{")
            
            for prop_name, prop_schema in schema.get('properties', {}).items():
                # Check for $ref BEFORE resolving, to get the reference name
                has_direct_ref = '$ref' in prop_schema
                ref_name = prop_schema['$ref'].split('/')[-1] if has_direct_ref else None
                
                prop_schema = getSchema_from_spec(spec, prop_schema)
                
                if prop_schema.get('type') == 'array':
                    items = prop_schema.get('items', {})
                    if '$ref' in items:
                        ref_name = items['$ref'].split('/')[-1]
                        go_type = f"[]{ref_name}"
                    elif 'type' in items:
                        item_type, _ = go_type_from_openapi_type(spec, items, set())
                        go_type = f"[]{item_type}"
                    else:
                        go_type = "[]interface{}"
                elif has_direct_ref and ref_name:
                    # Use the reference type name directly (don't treat as inline object)
                    go_type = ref_name
                elif prop_schema.get('additionalProperties'):
                    # Map type
                    additional = prop_schema.get('additionalProperties')
                    if '$ref' in additional:
                        value_type = additional['$ref'].split('/')[-1]
                    else:
                        value_type, _ = go_type_from_openapi_type(spec, additional, set())
                        if value_type is None:
                            value_type = 'interface{}'
                    go_type = f"map[string]{value_type}"
                elif prop_schema.get('type') == 'object' and 'properties' in prop_schema:
                    # Inline object - generate anonymous nested struct inline
                    nested_fields = generate_inline_struct_fields(spec, prop_schema, set())
                    go_type = f"struct {{ {nested_fields} }}"
                else:
                    go_type, _ = go_type_from_openapi_type(spec, prop_schema, set())
                
                if go_type is None:
                    go_type = 'interface{}'
                
                json_tag = to_lower_camel_case(prop_name)
                field_name = to_camel_case(prop_name)
                lines.append(f"\t{field_name} {go_type} `json:\"{json_tag}\"`")
            
            lines.append("}")
            lines.append("")
            generated_structs.add(struct_name)
    
    with open(models_path, "w") as f:
        f.write("\n".join(lines).rstrip() + "\n")
    
    print(f"Generated {models_path}")


def generate_resolver_file(endpoint: Dict[str, Any], spec: Dict[str, Any]) -> str:
    """Generate resolver file content"""
    handler_name = endpoint['handler']
    operation_id = endpoint.get('operationId', handler_name)
    exported_name = to_exported_ident(operation_id)
    file_name = to_snake_case(handler_name)
    
    request_schema = endpoint.get('request_schema')
    response_schema = endpoint.get('response_schema')
    has_request = request_schema is not None and request_schema.get('properties')
    
    imports = [
        'package api',
        '',
        'import (',
        '\t"context"',
        '\tapimodels "factorbacktest/api/models"',
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
\tvar requestBody apimodels.{exported_name}Request
\tif err := c.ShouldBindJSON(&requestBody); err != nil {{
\t\treturnErrorJson(err, c)
\t\treturn
\t}}
"""
    
    handler_func += """
\t// TODO: Implement handler logic

\tlg.Info("handler completed successfully")
\tc.JSON(200, apimodels.""" + exported_name + """Response{
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
    existing_paths = set()
    
    for i in range(route_start_idx, route_end_idx):
        line = lines[i]
        stripped = line.strip()
        if stripped.startswith('engine.') and ('GET(' in stripped or 'POST(' in stripped or 'PUT(' in stripped or 'DELETE(' in stripped):
            # Extract path from line
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
    insert_idx = route_end_idx
    if route_end_idx > 0 and lines[route_end_idx - 1].strip() == '':
        insert_idx = route_end_idx - 1
    
    # Build new file content
    new_lines = lines[:insert_idx] + new_routes + [''] + lines[route_end_idx:]
    
    with open(api_go_path, 'w') as f:
        f.writelines(new_lines)
    
    print(f"Updated {api_go_path} with {len(new_routes)} new route(s)")


def generate_resolver_files(endpoints: List[Dict[str, Any]], spec: Dict[str, Any]) -> None:
    """Generate resolver files for endpoints that don't exist"""
    for endpoint in endpoints:
        if endpoint.get("generate_resolver") is False:
            print(f"Skipping resolver generation for {endpoint.get('path')} (generate_resolver: false)")
            continue
        
        # Skip root endpoint (uses inline anonymous handler in api.go)
        operation_id = endpoint.get('operationId', endpoint.get('handler', ''))
        if operation_id == 'root':
            print(f"Skipping resolver generation for {endpoint.get('path')} (operationId: root)")
            continue
        
        handler_name = endpoint['handler']
        file_name = to_snake_case(handler_name)
        resolver_path = API_DIR / f"{file_name}.resolver.go"
        
        if resolver_path.exists():
            print(f"Skipping {resolver_path} (already exists)")
            continue
        
        content = generate_resolver_file(endpoint, spec)
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

'''
    
    # Track generated resources so deployments can be forced to redeploy when
    # routes change (API Gateway deployments are immutable snapshots).
    integration_resources = []
    endpoint_resource_names: List[str] = []
    
    # Generate resources for each endpoint
    for endpoint in endpoints:
        path = endpoint['path']
        method = endpoint['method'].upper()
        timeout = endpoint.get('x-aws-timeout', 30)
        integration_type = endpoint.get('x-aws-integration-type', 'lambda_proxy')
        
        # Convert path to Terraform-safe resource name
        resource_name = to_snake_case(path.lstrip('/').replace('/', '_'))
        
        # For simple paths (single segment), create resource directly
        path_parts = [p for p in path.split('/') if p]
        
        if len(path_parts) == 1:
            terraform_content += f'''
# Endpoint: {path} ({method})
resource "aws_api_gateway_resource" "{resource_name}" {{
  rest_api_id = var.api_gateway_rest_api_id
  parent_id   = local.root_resource_id
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
  timeout = {timeout}
}}

'''
            integration_resources.append(f'aws_api_gateway_integration.{resource_name}')
            endpoint_resource_names.append(resource_name)
        else:
            print(f"Warning: Nested paths not yet supported: {path}")
    
    # Generate deployment
    terraform_content += '''
# Deployment - creates a new deployment each time
# Note: Terraform will create a new deployment on every apply
# You may want to use aws_api_gateway_stage instead for better control
resource "aws_api_gateway_deployment" "main" {
  rest_api_id = var.api_gateway_rest_api_id

  depends_on = [
'''
    
    for integration in integration_resources:
        terraform_content += f'    {integration},\n'
    
    terraform_content += '''  ]

  # Force a new deployment when the API surface changes.
  # Without this, Terraform can add resources/methods/integrations without
  # creating a new deployment, leaving the stage pointing at an older snapshot.
  triggers = {
    redeployment = sha1(jsonencode([
'''
    
    for resource_name in endpoint_resource_names:
        terraform_content += (
            f'      aws_api_gateway_resource.{resource_name}.id,\n'
            f'      aws_api_gateway_method.{resource_name}.id,\n'
            f'      aws_api_gateway_integration.{resource_name}.id,\n'
        )
    
    terraform_content += '''    ]))
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
    print("\n  Note: Terraform files generated only. No changes applied to AWS.")


def generate_markdown_docs(spec: Dict[str, Any]) -> None:
    """Generate Markdown documentation from OpenAPI spec"""
    DOCS_DIR.mkdir(exist_ok=True)
    docs_path = DOCS_DIR / "api.md"
    
    lines = []
    lines.append(f"# {spec.get('info', {}).get('title', 'API Documentation')}")
    lines.append("")
    lines.append(f"Version: {spec.get('info', {}).get('version', '1.0.0')}")
    lines.append("")
    lines.append(f"Description: {spec.get('info', {}).get('description', '')}")
    lines.append("")
    
    # Paths
    lines.append("## Endpoints")
    lines.append("")
    
    paths = spec.get('paths', {})
    for path, path_item in paths.items():
        for method, operation in path_item.items():
            if method not in ['get', 'post', 'put', 'delete', 'patch']:
                continue
            
            operation_id = operation.get('operationId', 'unnamed')
            summary = operation.get('summary', '')
            description = operation.get('description', '')
            tags = operation.get('tags', [])
            
            lines.append(f"### {method.upper()} {path}")
            lines.append("")
            lines.append(f"**Operation ID:** {operation_id}")
            lines.append("")
            if summary:
                lines.append(f"**Summary:** {summary}")
                lines.append("")
            if description:
                lines.append(f"**Description:** {description}")
                lines.append("")
            if tags:
                lines.append(f"**Tags:** {', '.join(tags)}")
                lines.append("")
            
            # Security
            security = operation.get('security', [])
            if security:
                lines.append("**Security:** Requires authentication")
                lines.append("")
            
            # Request body
            request_body = operation.get('requestBody', {})
            if request_body:
                lines.append("**Request Body:**")
                lines.append("")
                content = request_body.get('content', {})
                if 'application/json' in content:
                    schema = content['application/json'].get('schema', {})
                    if '$ref' in schema:
                        schema_name = schema['$ref'].split('/')[-1]
                        lines.append(f"```json")
                        lines.append(f"{{\"$ref\": \"#/components/schemas/{schema_name}\"}}")
                        lines.append(f"```")
                    else:
                        lines.append(f"```json")
                        lines.append(json.dumps(schema, indent=2))
                        lines.append(f"```")
                    lines.append("")
            
            # Responses
            responses = operation.get('responses', {})
            if responses:
                lines.append("**Responses:**")
                lines.append("")
                for code, response in responses.items():
                    desc = response.get('description', '')
                    lines.append(f"- **{code}:** {desc}")
                lines.append("")
            
            lines.append("---")
            lines.append("")
    
    # Schemas
    lines.append("## Schemas")
    lines.append("")
    
    components = spec.get('components', {})
    schemas = components.get('schemas', {})
    
    for schema_name, schema in schemas.items():
        if not isinstance(schema, dict):
            continue
        
        lines.append(f"### {schema_name}")
        lines.append("")
        
        schema_type = schema.get('type', 'object')
        lines.append(f"**Type:** {schema_type}")
        lines.append("")
        
        description = schema.get('description', '')
        if description:
            lines.append(f"**Description:** {description}")
            lines.append("")
        
        properties = schema.get('properties', {})
        if properties:
            lines.append("**Properties:**")
            lines.append("")
            lines.append("| Name | Type | Format | Description |")
            lines.append("|------|------|--------|-------------|")
            
            for prop_name, prop_schema in properties.items():
                if isinstance(prop_schema, dict):
                    prop_type = prop_schema.get('type', 'object')
                    prop_format = prop_schema.get('format', '')
                    prop_desc = prop_schema.get('description', '')
                    prop_required = prop_schema.get('required', [])
                    
                    desc_text = prop_desc
                    if prop_name in prop_required:
                        desc_text = desc_text + " (required)" if desc_text else "(required)"
                    
                    lines.append(f"| {prop_name} | {prop_type} | {prop_format} | {desc_text} |")
                else:
                    lines.append(f"| {prop_name} | {prop_schema} | | |")
            lines.append("")
        
        lines.append("---")
        lines.append("")
    
    with open(docs_path, 'w') as f:
        f.write("\n".join(lines))
    
    print(f"Generated {docs_path}")


def parse_openapi_spec(spec: Dict[str, Any]) -> List[Dict[str, Any]]:
    """Parse OpenAPI spec and extract endpoint information"""
    endpoints = []
    
    paths = spec.get('paths', {})
    
    for path, path_item in paths.items():
        for method in ['get', 'post', 'put', 'delete', 'patch']:
            if method not in path_item:
                continue
            
            operation = path_item[method]
            
            # Get handler name from x-handler or operationId
            handler = operation.get('x-handler')
            if not handler:
                operation_id = operation.get('operationId')
                if operation_id:
                    handler = operation_id
            
            if not handler:
                continue
            
            endpoint = {
                'path': path,
                'method': method,
                'handler': handler,
                'operationId': operation.get('operationId', handler),
                'x-aws-integration-type': operation.get('x-aws-integration-type', 'lambda_proxy'),
                'x-aws-timeout': operation.get('x-aws-timeout', 30),
                'generate_resolver': operation.get('x-generate-resolver', True),
                'security': operation.get('security', []),
            }
            
            # Parse request body
            request_body = operation.get('requestBody', {})
            if request_body:
                content = request_body.get('content', {})
                if 'application/json' in content:
                    schema = content['application/json'].get('schema', {})
                    endpoint['request_schema'] = schema
                else:
                    endpoint['request_schema'] = {}
            else:
                endpoint['request_schema'] = None
            
            # Parse response
            responses = operation.get('responses', {})
            if '200' in responses:
                response = responses['200']
                content = response.get('content', {})
                if 'application/json' in content:
                    schema = content['application/json'].get('schema', {})
                    endpoint['response_schema'] = schema
                else:
                    endpoint['response_schema'] = {}
            else:
                endpoint['response_schema'] = {}
            
            endpoints.append(endpoint)
    
    return endpoints


def main():
    """Main entry point"""
    if not OPENAPI_YAML.exists():
        print(f"Error: {OPENAPI_YAML} not found")
        return 1
    
    with open(OPENAPI_YAML, 'r') as f:
        spec = yaml.safe_load(f)
    
    if not spec:
        print("Error: Could not parse OpenAPI spec")
        return 1
    
    endpoints = parse_openapi_spec(spec)
    
    if not endpoints:
        print("No endpoints found in OpenAPI spec")
        return 1
    
    print(f"Processing {len(endpoints)} endpoint(s)...")
    
    # Generate shared models
    generate_models_file(endpoints, spec)
    
    # Generate resolver files
    generate_resolver_files(endpoints, spec)
    
    # Update api.go
    update_api_go(endpoints)
    
    # Generate Terraform
    generate_terraform(endpoints)
    
    # Generate Markdown docs
    generate_markdown_docs(spec)
    
    print("\nDone! Generated files only - no changes applied.")
    print("\nNext steps:")
    print("1. Review generated resolver files")
    print("2. Implement handler logic in resolver files")
    print("3. Review terraform/api_gateway.tf and update with your API Gateway ID")
    print("4. Review docs/api.md for API documentation")
    print("5. To deploy: terraform -chdir=terraform init && terraform -chdir=terraform plan && terraform -chdir=terraform apply")
    
    return 0


if __name__ == '__main__':
    exit(main())
