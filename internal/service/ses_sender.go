package service

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SESSender sends email via Amazon SES.
type SESSender struct {
	client *sesv2.Client
}

// NewSESSender creates a new SESSender with the given AWS credentials.
func NewSESSender(accessKey, secretKey, region string) (*SESSender, error) {
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("AWS credentials are required")
	}

	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	cfg := aws.Config{
		Region:      region,
		Credentials: creds,
	}

	client := sesv2.NewFromConfig(cfg)
	return &SESSender{client: client}, nil
}

// SendRawEmail sends an email via Amazon SES.
func (s *SESSender) SendRawEmail(from, to, subject, body string) error {
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
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
					Text: &types.Content{
						Data:    aws.String(body),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	_, err := s.client.SendEmail(context.Background(), input)
	return err
}
