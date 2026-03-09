package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSConfig holds AWS SNS configuration.
type SNSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// SNSService implements PushSender using AWS SNS.
type SNSService struct {
	client *sns.Client
}

// NewSNSService creates a new SNS push notification sender.
func NewSNSService(ctx context.Context, cfg SNSConfig) (*SNSService, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &SNSService{
		client: sns.NewFromConfig(awsCfg),
	}, nil
}

// apnsPayload represents the Apple Push Notification Service payload structure.
type apnsPayload struct {
	APS  apnsAPS           `json:"aps"`
	Data map[string]string `json:"data,omitempty"`
}

type apnsAPS struct {
	Alert apnsAlert `json:"alert"`
	Badge int       `json:"badge,omitempty"`
	Sound string    `json:"sound,omitempty"`
}

type apnsAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// fcmPayload represents the Firebase Cloud Messaging payload structure.
type fcmPayload struct {
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	ImageURL string `json:"image,omitempty"`
}

// SendPush sends a push notification to a device via its SNS endpoint ARN.
func (s *SNSService) SendPush(ctx context.Context, endpointARN string, msg PushMessage) error {
	// Build multi-platform message
	apns := apnsPayload{
		APS: apnsAPS{
			Alert: apnsAlert{
				Title: msg.Title,
				Body:  msg.Body,
			},
			Badge: msg.Badge,
			Sound: "default",
		},
		Data: msg.Data,
	}

	fcm := fcmPayload{
		Notification: fcmNotification{
			Title:    msg.Title,
			Body:     msg.Body,
			ImageURL: msg.ImageURL,
		},
		Data: msg.Data,
	}

	apnsJSON, err := json.Marshal(apns)
	if err != nil {
		return fmt.Errorf("marshal apns: %w", err)
	}

	fcmJSON, err := json.Marshal(fcm)
	if err != nil {
		return fmt.Errorf("marshal fcm: %w", err)
	}

	// SNS requires a multi-platform message structure
	message := map[string]string{
		"default":      msg.Body,
		"APNS":         string(apnsJSON),
		"APNS_SANDBOX": string(apnsJSON),
		"GCM":          string(fcmJSON),
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	_, err = s.client.Publish(ctx, &sns.PublishInput{
		TargetArn:        aws.String(endpointARN),
		Message:          aws.String(string(messageJSON)),
		MessageStructure: aws.String("json"),
	})
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

// CreatePlatformEndpoint registers a device token with SNS and returns the endpoint ARN.
func (s *SNSService) CreatePlatformEndpoint(ctx context.Context, platformAppARN, token string) (string, error) {
	output, err := s.client.CreatePlatformEndpoint(ctx, &sns.CreatePlatformEndpointInput{
		PlatformApplicationArn: aws.String(platformAppARN),
		Token:                  aws.String(token),
	})
	if err != nil {
		return "", fmt.Errorf("create platform endpoint: %w", err)
	}
	return *output.EndpointArn, nil
}

// DeletePlatformEndpoint removes a device endpoint from SNS.
func (s *SNSService) DeletePlatformEndpoint(ctx context.Context, endpointARN string) error {
	_, err := s.client.DeleteEndpoint(ctx, &sns.DeleteEndpointInput{
		EndpointArn: aws.String(endpointARN),
	})
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	return nil
}
