package user

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// User is the domain model used across service and repository layers.
// Neither SQLC nor GORM types leak outside the repository.
type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	FullName     string
	PasswordHash string
	Role         string
	IsActive     bool
	Metadata     json.RawMessage
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}
