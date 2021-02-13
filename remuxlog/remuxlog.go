package remuxlog

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	log "github.com/sirupsen/logrus"
)

type RemuxLog interface {
	LogRemux(inputKey string, length int, requestId string, outputKey string, error string) error
}

type dynamoLogger struct {
	tableName    *string
	dynamoClient *dynamodb.DynamoDB
}

func New(tableName string) RemuxLog {
	dynamo := dynamodb.New(session.Must(session.NewSession()))
	return &dynamoLogger{
		tableName:    aws.String(tableName),
		dynamoClient: dynamo,
	}
}

func (dl *dynamoLogger) LogRemux(inputKey string, length int, requestId string, outputKey string, error string) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	durationString := fmt.Sprintf("%d", length)

	_, err := dl.dynamoClient.PutItem(&dynamodb.PutItemInput{
		TableName: dl.tableName,
		Item: map[string]*dynamodb.AttributeValue{
			"input_key":  {S: &inputKey},
			"timestamp":  {N: &timestamp},
			"duration":   {N: &durationString},
			"request_id": {S: &requestId},
			"output_key": {S: &outputKey},
			"successful": {BOOL: aws.Bool(error == "")},
			"error":      {S: &error},
		},
	})

	if err != nil {
		log.WithError(err).Warn("unable to store encode record")
	}

	return err
}
