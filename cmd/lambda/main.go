package main

import (
	"context"
	"factorbacktest/api"
	"factorbacktest/cmd"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
)

type lambdaHandler struct {
	apiHandler *api.ApiHandler
}

func (m lambdaHandler) Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	engine := m.apiHandler.InitializeRouterEngine()
	ginLambda := ginadapter.New(engine)

	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	apiHandler, err := cmd.InitializeDependencies()
	if err != nil {
		log.Fatal(err)
	}
	handler := lambdaHandler{
		apiHandler: apiHandler,
	}
	defer cmd.CloseDependencies(apiHandler)
	// TODO - double check where i should close db conn
	lambda.Start(handler.Handler)
}
