package sendgrid

// Address represents an email address with an optional display name.
type Address struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// Personalization defines recipients for a single personalization block.
type Personalization struct {
	To  []Address `json:"to"`
	CC  []Address `json:"cc,omitempty"`
	BCC []Address `json:"bcc,omitempty"`
}

// Content represents the body content of an email.
type Content struct {
	Type  string `json:"type"`  // e.g. "text/plain", "text/html"
	Value string `json:"value"`
}

// SendRequest is the payload for the SendGrid v3 mail/send endpoint.
type SendRequest struct {
	Personalizations []Personalization `json:"personalizations"`
	From             Address           `json:"from"`
	Subject          string            `json:"subject"`
	Content          []Content         `json:"content"`
}
