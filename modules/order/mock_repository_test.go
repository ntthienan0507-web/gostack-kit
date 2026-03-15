package order

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// MockRepository is a testify mock for the Repository interface.
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) List(ctx context.Context, params ListParams, limit, offset int32) ([]*Order, error) {
	args := m.Called(ctx, params, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Order), args.Error(1)
}

func (m *MockRepository) Count(ctx context.Context, params ListParams) (int64, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) GetByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Order), args.Error(1)
}

func (m *MockRepository) Create(ctx context.Context, tx *gorm.DB, order *Order) (*Order, error) {
	args := m.Called(ctx, tx, order)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Order), args.Error(1)
}

func (m *MockRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, status OrderStatus, version int) error {
	args := m.Called(ctx, tx, id, status, version)
	return args.Error(0)
}

func (m *MockRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
