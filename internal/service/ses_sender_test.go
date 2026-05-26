package service

import (
	"strings"
	"testing"
)

func TestNewSESSender_ValidCredentials(t *testing.T) {
	sender, err := NewSESSender("TEST_AWS_ACCESS_KEY", "TEST_AWS_SECRET_KEY", "us-east-1")
	if err != nil {
		t.Errorf("NewSESSender() with valid credentials returned error: %v", err)
	}
	if sender == nil {
		t.Error("NewSESSender() returned nil sender")
	}
	if sender != nil && sender.client == nil {
		t.Error("NewSESSender() returned sender with nil client")
	}
}

func TestNewSESSender_EmptyAccessKey(t *testing.T) {
	_, err := NewSESSender("", "TEST_AWS_SECRET_KEY", "us-east-1")
	if err == nil {
		t.Error("NewSESSender() with empty access key should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "credentials are required") {
		t.Errorf("NewSESSender() error = %q, want error about credentials", err.Error())
	}
}

func TestNewSESSender_EmptySecretKey(t *testing.T) {
	_, err := NewSESSender("TEST_AWS_ACCESS_KEY", "", "us-east-1")
	if err == nil {
		t.Error("NewSESSender() with empty secret key should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "credentials are required") {
		t.Errorf("NewSESSender() error = %q, want error about credentials", err.Error())
	}
}

func TestNewSESSender_BothCredentialsEmpty(t *testing.T) {
	_, err := NewSESSender("", "", "us-east-1")
	if err == nil {
		t.Error("NewSESSender() with both credentials empty should return error")
	}
}

func TestNewSESSender_DifferentRegions(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			sender, err := NewSESSender("TEST_AWS_ACCESS_KEY", "TEST_AWS_SECRET_KEY", region)
			if err != nil {
				t.Errorf("NewSESSender() with region %q returned error: %v", region, err)
			}
			if sender == nil {
				t.Errorf("NewSESSender() with region %q returned nil sender", region)
			}
		})
	}
}

func TestSESSender_ImplementsEmailSender(t *testing.T) {
	sender, err := NewSESSender("TEST_AWS_ACCESS_KEY", "secret", "us-east-1")
	if err != nil {
		t.Fatalf("NewSESSender() returned error: %v", err)
	}

	// Verify SESSender implements EmailSender interface
	var _ EmailSender = sender
}
