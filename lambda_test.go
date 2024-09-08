package main

import (
	"context"
	_ "embed"
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
	sentParams []ses.SendEmailInput
}

func (m *MockSESClient) SendEmail(ctx context.Context, params *ses.SendEmailInput, optFns ...func(*ses.Options)) (*ses.SendEmailOutput, error) {
	m.sentEmails++
	m.sentParams = append(m.sentParams, *params)
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
	messageText := mockSESClient.sentParams[0].Message.Body.Text.Data
	if messageText == nil {
		t.Fatalf("Expected email body to be text")
	}
	if *messageText != "This is a test email body." {
		t.Errorf("Expected email body to be \"This is a test email body.\", but got \"%s\"", *messageText)
	}
}

//go:embed testdata/20240908-005307-multipart-email.eml
var testMultipartEmail1 string

type MultipartEmailMock1 struct{}

func (m *MultipartEmailMock1) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	// Return a mock email content
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(testMultipartEmail1)),
	}, nil
}

func TestMultipartEmail(t *testing.T) {
	t.Setenv("FORWARD_TO_ADDRESS", "mail@example.com")

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

	mockSESClient := &MockSESClient{}
	mockS3Client := &MultipartEmailMock1{}

	err := handleRequest(context.Background(), s3Event, mockS3Client, mockSESClient)
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	if mockSESClient.sentEmails != 1 {
		t.Errorf("Expected 1 email to be sent, but got %d", mockSESClient.sentEmails)
	}
	messageText := mockSESClient.sentParams[0].Message.Body.Html.Data
	if messageText == nil {
		t.Fatalf("Expected email body to be HTML")
	}
	if !strings.Contains(*messageText, "convolutional") {
		t.Errorf("Unexpected email body: \"%s\"", *messageText)
	}
}
