package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// EmailRepository is responsible for sending emails.
// It's a thin wrapper around AWS SES - it only sends pre-rendered HTML.
// Template rendering is handled by EmailService.
type EmailRepository interface {
	// SendEmail sends an email to the specified recipient
	// with the given subject and body (HTML or plain text)
	SendEmail(to string, subject string, body string) error
}

type emailRepositoryHandler struct {
	sesClient *sesv2.Client
	fromEmail string
}

// NewEmailRepository creates a new email repository using AWS SES.
// region should be the AWS region (e.g., "us-east-1")
// fromEmail should be the verified sender email (e.g., "noreply@factor.trade")
func NewEmailRepository(region, fromEmail string) (EmailRepository, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sesv2.NewFromConfig(cfg)

	return &emailRepositoryHandler{
		sesClient: client,
		fromEmail: fromEmail,
	}, nil
}

func (h *emailRepositoryHandler) SendEmail(to string, subject string, body string) error {
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(h.fromEmail),
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data:    aws.String(body),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	result, err := h.sesClient.SendEmail(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to send email via SES: %w", err)
	}

	// Log the message ID for tracking (optional - can be removed if not needed)
	if result.MessageId != nil {
		// Message ID can be used to track email delivery in CloudWatch
		_ = result.MessageId
	}

	return nil
}
