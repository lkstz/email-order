package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(Handler)
	//if err := PlaceOrder(); err != nil {
	//	fmt.Println(err)
	//}
}

func Handler(_ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if err := PlaceOrder(); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}
