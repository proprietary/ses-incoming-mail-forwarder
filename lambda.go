package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
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
	forwardToAddresses := splitCsv(os.Getenv("FORWARD_TO_ADDRESS"))

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

		from := msg.Header.Get("From")
		to := msg.Header.Get("To")

		m, err := convertMessage(msg)
		if err != nil {
			return fmt.Errorf("failed to convert message: %v", err)
		}

		// Send the email using SES
		_, err = sesClient.SendEmail(ctx, &ses.SendEmailInput{
			Destination: &types.Destination{
				ToAddresses: forwardToAddresses,
			},
			Message:          &m,
			Source:           &to,
			ReplyToAddresses: []string{from},
		})
		if err != nil {
			return fmt.Errorf("failed to send email: %v", err)
		}

		log.Printf("Successfully forwarded email from %s to %s", from, forwardToAddresses)
	}

	return nil
}

func splitCsv(s string) []string {
	return strings.Split(s, ",")
}

func convertMessage(m *mail.Message) (types.Message, error) {
	subject := m.Header.Get("Subject")

	// read potentially multipart message
	var body *types.Body
	mediaType, params, _ := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if strings.HasPrefix(mediaType, "multipart/") {
		parts, err := parsePart(m.Body, params["boundary"])
		if err != nil {
			return types.Message{}, fmt.Errorf("failed to parse parts: %v", err)
		}
		var bodyText strings.Builder
		for _, part := range parts {
			bodyText.WriteString(part)
		}
		bodyAsText := bodyText.String()
		body = &types.Body{
			Html: &types.Content{
				Data: &bodyAsText,
			},
		}
	} else {
		var bodyText strings.Builder
		_, err := io.Copy(&bodyText, m.Body)
		if err != nil {
			return types.Message{}, fmt.Errorf("failed to read body: %v", err)
		}
		bodyAsText := bodyText.String()
		body = &types.Body{
			Text: &types.Content{
				Data: &bodyAsText,
			},
		}
	}

	out := types.Message{
		Subject: &types.Content{
			Data: &subject,
		},
		Body: body,
	}

	return out, nil
}

func parsePart(src io.Reader, boundary string) ([]string, error) {
	mr := multipart.NewReader(src, boundary)
	var parts []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read part: %v", err)
		}
		mediaType, params, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if strings.HasPrefix(mediaType, "multipart/") {
			subParts, err := parsePart(p, params["boundary"])
			if err != nil {
				return nil, fmt.Errorf("failed to parse subpart: %v", err)
			}
			parts = append(parts, subParts...)
		} else {
			decoded, err := decodePart(p)
			if err != nil {
				return nil, fmt.Errorf("failed to decode part: %v", err)
			}
			parts = append(parts, string(decoded))
		}
	}
	return parts, nil
}

func decodePart(src *multipart.Part) (string, error) {
	contentTransferEncoding := strings.ToUpper(src.Header.Get("Content-Transfer-Encoding"))
	switch {
	case strings.Compare(contentTransferEncoding, "BASE64") == 0: // base64
		buf, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, src))
		if err != nil {
			return "", fmt.Errorf("failed to decode base64: %v", err)
		}
		return string(buf), nil
	case strings.Compare(contentTransferEncoding, "QUOTED-PRINTABLE") == 0: // quoted-printable
		buf, err := io.ReadAll(quotedprintable.NewReader(src))
		if err != nil {
			return "", fmt.Errorf("failed to decode quoted-printable: %v", err)
		}
		return string(buf), nil
	default:
		buf, err := io.ReadAll(src)
		if err != nil {
			return "", fmt.Errorf("failed to read part: %v", err)
		}
		return string(buf), nil
	}
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
