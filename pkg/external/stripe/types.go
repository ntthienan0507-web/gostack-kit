package stripe

// ChargeRequest represents a request to create a charge.
type ChargeRequest struct {
	Amount      int    `json:"amount"`      // amount in smallest currency unit (e.g. cents)
	Currency    string `json:"currency"`    // three-letter ISO currency code
	Source      string `json:"source"`      // payment source token
	Description string `json:"description"` // optional description
}

// ChargeResponse represents the response from creating a charge.
type ChargeResponse struct {
	ID       string `json:"id"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Paid     bool   `json:"paid"`
}

// RefundRequest represents a request to refund a charge.
type RefundRequest struct {
	ChargeID string `json:"charge"`
	Amount   int    `json:"amount,omitempty"` // partial refund amount; 0 = full refund
}

// RefundResponse represents the response from creating a refund.
type RefundResponse struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
	Status string `json:"status"`
}
