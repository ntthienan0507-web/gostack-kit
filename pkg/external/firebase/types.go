package firebase

// PushNotification represents a push notification to a single device.
type PushNotification struct {
	Token    string            `json:"token"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	ImageURL string            `json:"image_url,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

// MulticastNotification represents a push notification to multiple devices.
type MulticastNotification struct {
	Tokens   []string          `json:"tokens"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	ImageURL string            `json:"image_url,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

// TopicNotification represents a push notification to a topic.
type TopicNotification struct {
	Topic    string            `json:"topic"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	ImageURL string            `json:"image_url,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

// SendResult represents the result of a multicast send operation.
type SendResult struct {
	SuccessCount  int      `json:"success_count"`
	FailureCount  int      `json:"failure_count"`
	InvalidTokens []string `json:"invalid_tokens,omitempty"`
}
