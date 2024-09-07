package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type S3ClientInterface interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type SESClientInterface interface {
	SendEmail(ctx context.Context, params *ses.SendEmailInput, optFns ...func(*ses.Options)) (*ses.SendEmailOutput, error)
}

func handleRequest(ctx context.Context, s3Event events.S3Event, s3Client S3ClientInterface, sesClient SESClientInterface) error {
	var forwardToAddress string = os.Getenv("FORWARD_TO_ADDRESS")

	for _, record := range s3Event.Records {
		bucket := record.S3.Bucket.Name
		key := record.S3.Object.Key

		obj, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			return fmt.Errorf("failed to get object from S3: %v", err)
		}

		content, err := io.ReadAll(obj.Body)
		if err != nil {
			return fmt.Errorf("failed to read object body: %v", err)
		}

		msg, err := mail.ReadMessage(strings.NewReader(string(content)))
		if err != nil {
			return fmt.Errorf("failed to parse email: %v", err)
		}

		subject := msg.Header.Get("Subject")
		from := msg.Header.Get("From")
		to := msg.Header.Get("To")

		var bodyBuilder strings.Builder
		bodyBuilder.WriteString(fmt.Sprintf("From: %s\n", from))
		bodyBuilder.WriteString(fmt.Sprintf("Subject: %s\n\n", subject))
		bodyBuilder.WriteString("--- Original message ---\n\n")
		io.Copy(&bodyBuilder, msg.Body)
		bodyAsString := bodyBuilder.String()

		// Send the email using SES
		_, err = sesClient.SendEmail(ctx, &ses.SendEmailInput{
			Destination: &types.Destination{
				ToAddresses: []string{forwardToAddress},
			},
			Message: &types.Message{
				Body: &types.Body{
					Text: &types.Content{
						Data: &bodyAsString,
					},
				},
				Subject: &types.Content{
					Data: &subject,
				},
			},
			Source:           &to,
			ReplyToAddresses: []string{from},
		})
		if err != nil {
			return fmt.Errorf("failed to send email: %v", err)
		}

		log.Printf("Successfully forwarded email from %s to %s", from, forwardToAddress)
	}

	return nil
}

func main() {
	lambda.Start(func(ctx context.Context, s3Event events.S3Event) error {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %v", err)
		}

		s3Client := s3.NewFromConfig(cfg)
		sesClient := ses.NewFromConfig(cfg)

		return handleRequest(ctx, s3Event, s3Client, sesClient)
	})
}
