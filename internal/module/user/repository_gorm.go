package user

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// gormUser is the GORM model for the users table.
// Maps to the same PostgreSQL table as SQLC — schema managed by goose migrations.
type gormUser struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Username     string          `gorm:"type:varchar(100);uniqueIndex;not null"`
	Email        string          `gorm:"type:varchar(255);uniqueIndex;not null"`
	FullName     string          `gorm:"type:varchar(255);not null"`
	PasswordHash *string         `gorm:"type:varchar(255)"`
	Role         string          `gorm:"type:varchar(50);not null;default:'user'"`
	IsActive     bool            `gorm:"not null;default:true"`
	Metadata     json.RawMessage `gorm:"type:jsonb;default:'{}'"`
	CreatedAt    time.Time       `gorm:"autoCreateTime"`
	UpdatedAt    time.Time       `gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt  `gorm:"index"`
}

// TableName tells GORM the table name (matches goose migration).
func (gormUser) TableName() string { return "users" }

// gormRepository implements Repository using GORM.
type gormRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a GORM-backed user repository.
func NewGORMRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) List(ctx context.Context, params ListParams, limit, offset int32) ([]*User, error) {
	var rows []gormUser
	q := r.db.WithContext(ctx).Where("deleted_at IS NULL")

	if params.Search != "" {
		pattern := "%" + params.Search + "%"
		q = q.Where("full_name ILIKE ? OR email ILIKE ? OR username ILIKE ?", pattern, pattern, pattern)
	}
	if params.Role != "" {
		q = q.Where("role = ?", params.Role)
	}

	err := q.Order("created_at DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	return mapSlice(rows, fromGormUser), nil
}

func (r *gormRepository) Count(ctx context.Context, params ListParams) (int64, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&gormUser{}).Where("deleted_at IS NULL")

	if params.Search != "" {
		pattern := "%" + params.Search + "%"
		q = q.Where("full_name ILIKE ? OR email ILIKE ? OR username ILIKE ?", pattern, pattern, pattern)
	}
	if params.Role != "" {
		q = q.Where("role = ?", params.Role)
	}

	err := q.Count(&count).Error
	return count, err
}

func (r *gormRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var row gormUser
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&row).Error
	if err != nil {
		return nil, err
	}
	u := fromGormUserVal(row)
	return u, nil
}

func (r *gormRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var row gormUser
	err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&row).Error
	if err != nil {
		return nil, err
	}
	u := fromGormUserVal(row)
	return u, nil
}

func (r *gormRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	var row gormUser
	err := r.db.WithContext(ctx).Where("username = ? AND deleted_at IS NULL", username).First(&row).Error
	if err != nil {
		return nil, err
	}
	u := fromGormUserVal(row)
	return u, nil
}

func (r *gormRepository) Create(ctx context.Context, input CreateInput) (*User, error) {
	row := gormUser{
		ID:       uuid.New(),
		Username: input.Username,
		Email:    input.Email,
		FullName: input.FullName,
		Role:     input.Role,
		IsActive: true,
	}
	if input.PasswordHash != "" {
		row.PasswordHash = &input.PasswordHash
	}

	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	u := fromGormUserVal(row)
	return u, nil
}

func (r *gormRepository) Update(ctx context.Context, input UpdateInput) (*User, error) {
	updates := map[string]interface{}{
		"full_name":  input.FullName,
		"role":       input.Role,
		"is_active":  input.IsActive,
		"updated_at": time.Now(),
	}

	var row gormUser
	err := r.db.WithContext(ctx).
		Model(&row).
		Where("id = ? AND deleted_at IS NULL", input.ID).
		Updates(updates).Error
	if err != nil {
		return nil, err
	}

	// Reload to get full record
	err = r.db.WithContext(ctx).Where("id = ?", input.ID).First(&row).Error
	if err != nil {
		return nil, err
	}
	u := fromGormUserVal(row)
	return u, nil
}

func (r *gormRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&gormUser{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// --- Mapping: gormUser → domain User ---

func fromGormUserVal(row gormUser) *User {
	u := &User{
		ID:        row.ID,
		Username:  row.Username,
		Email:     row.Email,
		FullName:  row.FullName,
		Role:      row.Role,
		IsActive:  row.IsActive,
		Metadata:  row.Metadata,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if row.PasswordHash != nil {
		u.PasswordHash = *row.PasswordHash
	}
	if row.DeletedAt.Valid {
		u.DeletedAt = &row.DeletedAt.Time
	}
	return u
}

// fromGormUser adapts gormUser (value) for mapSlice.
func fromGormUser(row gormUser) *User {
	return fromGormUserVal(row)
}
