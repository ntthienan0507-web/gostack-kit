package ws

import "encoding/json"

const (
	// TypeError is the message type for error messages.
	TypeError = "error"
	// TypePong is the message type for pong messages.
	TypePong = "pong"
)

// Message represents a WebSocket message exchanged between client and server.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// NewMessage creates a new Message with the given type and payload.
// The payload is marshaled to JSON. If marshaling fails, the payload is set to null.
func NewMessage(msgType string, payload any) Message {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			raw = json.RawMessage("null")
		} else {
			raw = data
		}
	}
	return Message{
		Type:    msgType,
		Payload: raw,
	}
}

// Decode unmarshals the message payload into the provided destination.
func (m Message) Decode(dest any) error {
	return json.Unmarshal(m.Payload, dest)
}
