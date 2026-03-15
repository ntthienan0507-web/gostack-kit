package firebase

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

// Config holds Firebase client configuration.
type Config struct {
	CredentialsFile string // path to the Firebase service account JSON file
	ProjectID       string
}

// Client wraps Firebase services for push notifications and auth.
type Client struct {
	app       *firebase.App
	auth      *auth.Client
	messaging *messaging.Client
	logger    *zap.Logger
}

// New creates a new Firebase client. It reads the credentials file and initializes
// the Firebase app, auth, and messaging clients.
func New(ctx context.Context, cfg Config, logger *zap.Logger) (*Client, error) {
	creds, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInitFailed, err)
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: cfg.ProjectID,
	}, option.WithCredentialsJSON(creds))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInitFailed, err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: auth client: %v", ErrInitFailed, err)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: messaging client: %v", ErrInitFailed, err)
	}

	logger.Info("firebase initialized", zap.String("project_id", cfg.ProjectID))

	return &Client{
		app:       app,
		auth:      authClient,
		messaging: msgClient,
		logger:    logger,
	}, nil
}

// SendPush sends a push notification to a single device token.
func (c *Client) SendPush(ctx context.Context, n PushNotification) (string, error) {
	msg := &messaging.Message{
		Token: n.Token,
		Notification: &messaging.Notification{
			Title:    n.Title,
			Body:     n.Body,
			ImageURL: n.ImageURL,
		},
		Data: n.Data,
	}

	messageID, err := c.messaging.Send(ctx, msg)
	if err != nil {
		c.logger.Error("firebase push failed",
			zap.String("token", n.Token),
			zap.Error(err),
		)
		return "", fmt.Errorf("%w: %v", ErrPushFailed, err)
	}

	return messageID, nil
}

// SendMulticast sends a push notification to multiple device tokens.
// It returns a SendResult with success/failure counts and any invalid tokens.
func (c *Client) SendMulticast(ctx context.Context, n MulticastNotification) (*SendResult, error) {
	msg := &messaging.MulticastMessage{
		Tokens: n.Tokens,
		Notification: &messaging.Notification{
			Title:    n.Title,
			Body:     n.Body,
			ImageURL: n.ImageURL,
		},
		Data: n.Data,
	}

	resp, err := c.messaging.SendEachForMulticast(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPushFailed, err)
	}

	result := &SendResult{
		SuccessCount: resp.SuccessCount,
		FailureCount: resp.FailureCount,
	}

	// Collect invalid (unregistered) tokens so callers can clean up.
	for i, sr := range resp.Responses {
		if sr.Error != nil && messaging.IsUnregistered(sr.Error) {
			result.InvalidTokens = append(result.InvalidTokens, n.Tokens[i])
		}
	}

	if len(result.InvalidTokens) > 0 {
		c.logger.Warn("firebase multicast: unregistered tokens detected",
			zap.Int("count", len(result.InvalidTokens)),
		)
	}

	return result, nil
}

// SendToTopic sends a push notification to all subscribers of a topic.
func (c *Client) SendToTopic(ctx context.Context, n TopicNotification) (string, error) {
	msg := &messaging.Message{
		Topic: n.Topic,
		Notification: &messaging.Notification{
			Title:    n.Title,
			Body:     n.Body,
			ImageURL: n.ImageURL,
		},
		Data: n.Data,
	}

	messageID, err := c.messaging.Send(ctx, msg)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPushFailed, err)
	}

	return messageID, nil
}

// VerifyIDToken verifies a Firebase ID token and returns the decoded token.
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := c.auth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVerifyFailed, err)
	}
	return token, nil
}

// SetCustomClaims sets custom claims on a user's token.
func (c *Client) SetCustomClaims(ctx context.Context, uid string, claims map[string]any) error {
	if err := c.auth.SetCustomUserClaims(ctx, uid, claims); err != nil {
		return fmt.Errorf("firebase set custom claims: %w", err)
	}
	return nil
}
