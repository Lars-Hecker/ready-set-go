package notification

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// SESConfig holds AWS SES configuration.
type SESConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	FromAddress     string
}

// SESService implements EmailSender using AWS SES.
type SESService struct {
	client      *ses.Client
	fromAddress string
}

// NewSESService creates a new SES email sender.
func NewSESService(ctx context.Context, cfg SESConfig) (*SESService, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &SESService{
		client:      ses.NewFromConfig(awsCfg),
		fromAddress: cfg.FromAddress,
	}, nil
}

// SendEmail sends an email via AWS SES.
func (s *SESService) SendEmail(ctx context.Context, msg EmailMessage) error {
	input := &ses.SendEmailInput{
		Source: aws.String(s.fromAddress),
		Destination: &types.Destination{
			ToAddresses: []string{msg.To},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(msg.Subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{},
		},
	}

	if msg.HTMLBody != "" {
		input.Message.Body.Html = &types.Content{
			Data:    aws.String(msg.HTMLBody),
			Charset: aws.String("UTF-8"),
		}
	}
	if msg.TextBody != "" {
		input.Message.Body.Text = &types.Content{
			Data:    aws.String(msg.TextBody),
			Charset: aws.String("UTF-8"),
		}
	}
	if msg.ReplyTo != "" {
		input.ReplyToAddresses = []string{msg.ReplyTo}
	}

	_, err := s.client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
