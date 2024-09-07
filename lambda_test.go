package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
)

type MockS3Client struct{}

func (m *MockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	// Return a mock email content
	mockEmail := `From: sender@example.com
To: recipient@example.com
Subject: Test Email

This is a test email body.`
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(mockEmail)),
	}, nil
}

type MockSESClient struct {
	sentEmails int
}

func (m *MockSESClient) SendEmail(ctx context.Context, params *ses.SendEmailInput, optFns ...func(*ses.Options)) (*ses.SendEmailOutput, error) {
	m.sentEmails++
	return &ses.SendEmailOutput{}, nil
}

func TestHandleRequest(t *testing.T) {
	t.Setenv("FORWARD_TO_ADDRESS", "mail@example.com")

	// Create a sample S3 event
	s3Event := events.S3Event{
		Records: []events.S3EventRecord{
			{
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-bucket"},
					Object: events.S3Object{Key: "test-key"},
				},
			},
		},
	}

	// Create mock clients
	mockS3Client := &MockS3Client{}
	mockSESClient := &MockSESClient{}

	// Call the handler
	err := handleRequest(context.Background(), s3Event, mockS3Client, mockSESClient)
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	// Check if an email was sent
	if mockSESClient.sentEmails != 1 {
		t.Errorf("Expected 1 email to be sent, but got %d", mockSESClient.sentEmails)
	}
}
