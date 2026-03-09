package config

import (
	"os"

	"baseapp/infra/notification"
)

// SESConfigFromEnv loads SES email configuration from environment variables.
func SESConfigFromEnv() notification.SESConfig {
	return notification.SESConfig{
		Region:          os.Getenv("SES_REGION"),
		AccessKeyID:     os.Getenv("SES_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("SES_SECRET_ACCESS_KEY"),
		FromAddress:     os.Getenv("SES_FROM_ADDRESS"),
	}
}

// SNSConfigFromEnv loads SNS push notification configuration from environment variables.
func SNSConfigFromEnv() notification.SNSConfig {
	return notification.SNSConfig{
		Region:          os.Getenv("SNS_REGION"),
		AccessKeyID:     os.Getenv("SNS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("SNS_SECRET_ACCESS_KEY"),
	}
}

// WebPushConfigFromEnv loads Web Push VAPID configuration from environment variables.
func WebPushConfigFromEnv() notification.WebPushConfig {
	return notification.WebPushConfig{
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    os.Getenv("VAPID_SUBJECT"),
	}
}
