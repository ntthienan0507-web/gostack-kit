package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestService() (*Service, *MockRepository) {
	repo := new(MockRepository)
	logger := zap.NewNop()
	svc := NewService(repo, nil, nil, nil, logger)
	return svc, repo
}

func sampleOrder() *Order {
	orderID := uuid.New()
	return &Order{
		ID:         orderID,
		UserID:     uuid.New(),
		Status:     StatusPending,
		TotalPrice: 59.98,
		Currency:   "USD",
		Note:       "Please deliver before noon",
		Version:    1,
		Items: []OrderItem{
			{
				ID:        uuid.New(),
				OrderID:   orderID,
				ProductID: uuid.New(),
				Name:      "Widget A",
				Quantity:  2,
				UnitPrice: 19.99,
				CreatedAt: time.Now(),
			},
			{
				ID:        uuid.New(),
				OrderID:   orderID,
				ProductID: uuid.New(),
				Name:      "Widget B",
				Quantity:  1,
				UnitPrice: 20.00,
				CreatedAt: time.Now(),
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// --- List ---

func TestService_List_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 1, PageSize: 20}

	orders := []*Order{sampleOrder(), sampleOrder()}
	repo.On("List", ctx, params, int32(20), int32(0)).Return(orders, nil)
	repo.On("Count", ctx, params).Return(int64(2), nil)

	result, err := svc.List(ctx, params)

	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.Total)
	repo.AssertExpectations(t)
}

func TestService_List_RepoListError(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 1, PageSize: 20}

	repo.On("List", ctx, params, int32(20), int32(0)).Return(nil, errors.New("db error"))

	result, err := svc.List(ctx, params)

	assert.Nil(t, result)
	assert.EqualError(t, err, "db error")
}

func TestService_List_RepoCountError(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 1, PageSize: 20}

	repo.On("List", ctx, params, int32(20), int32(0)).Return([]*Order{}, nil)
	repo.On("Count", ctx, params).Return(int64(0), errors.New("count error"))

	result, err := svc.List(ctx, params)

	assert.Nil(t, result)
	assert.EqualError(t, err, "count error")
}

func TestService_List_Pagination(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 3, PageSize: 10}

	repo.On("List", ctx, params, int32(10), int32(20)).Return([]*Order{}, nil)
	repo.On("Count", ctx, params).Return(int64(25), nil)

	result, err := svc.List(ctx, params)

	require.NoError(t, err)
	assert.Empty(t, result.Items)
	assert.Equal(t, int64(25), result.Total)
}

// --- GetByID ---

func TestService_GetByID_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	o := sampleOrder()

	repo.On("GetByID", ctx, o.ID).Return(o, nil)

	result, err := svc.GetByID(ctx, o.ID)

	require.NoError(t, err)
	assert.Equal(t, o.ID, result.ID)
	assert.Equal(t, o.UserID, result.UserID)
	assert.Equal(t, o.Status, result.Status)
	assert.Equal(t, o.TotalPrice, result.TotalPrice)
	assert.Equal(t, o.Currency, result.Currency)
	assert.Len(t, result.Items, 2)
}

func TestService_GetByID_NotFound(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, ErrOrderNotFound)

	result, err := svc.GetByID(ctx, id)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrOrderNotFound)
}

// --- Cancel (validation logic) ---

func TestService_Cancel_NotCancellable_Shipped(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	o := sampleOrder()
	o.Status = StatusShipped

	repo.On("GetByID", ctx, o.ID).Return(o, nil)

	err := svc.Cancel(ctx, o.ID)

	assert.ErrorIs(t, err, ErrNotCancellable)
}

func TestService_Cancel_NotCancellable_Delivered(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	o := sampleOrder()
	o.Status = StatusDelivered

	repo.On("GetByID", ctx, o.ID).Return(o, nil)

	err := svc.Cancel(ctx, o.ID)

	assert.ErrorIs(t, err, ErrNotCancellable)
}

func TestService_Cancel_NotCancellable_AlreadyCancelled(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	o := sampleOrder()
	o.Status = StatusCancelled

	repo.On("GetByID", ctx, o.ID).Return(o, nil)

	err := svc.Cancel(ctx, o.ID)

	assert.ErrorIs(t, err, ErrNotCancellable)
}

func TestService_Cancel_NotFound(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, ErrOrderNotFound)

	err := svc.Cancel(ctx, id)

	assert.ErrorIs(t, err, ErrOrderNotFound)
}

// --- ToResponse ---

func TestToResponse_MapsAllFields(t *testing.T) {
	o := sampleOrder()

	resp := ToResponse(o)

	assert.Equal(t, o.ID, resp.ID)
	assert.Equal(t, o.UserID, resp.UserID)
	assert.Equal(t, o.Status, resp.Status)
	assert.Equal(t, o.TotalPrice, resp.TotalPrice)
	assert.Equal(t, o.Currency, resp.Currency)
	assert.Equal(t, o.Note, resp.Note)
	assert.Equal(t, o.Version, resp.Version)
	assert.Equal(t, o.CreatedAt, resp.CreatedAt)
	assert.Equal(t, o.UpdatedAt, resp.UpdatedAt)
}

func TestToResponse_ItemsMappedCorrectly(t *testing.T) {
	o := sampleOrder()

	resp := ToResponse(o)

	require.Len(t, resp.Items, len(o.Items))
	for i, item := range o.Items {
		assert.Equal(t, item.ID, resp.Items[i].ID)
		assert.Equal(t, item.ProductID, resp.Items[i].ProductID)
		assert.Equal(t, item.Name, resp.Items[i].Name)
		assert.Equal(t, item.Quantity, resp.Items[i].Quantity)
		assert.Equal(t, item.UnitPrice, resp.Items[i].UnitPrice)
	}
}

func TestToResponse_EmptyItems(t *testing.T) {
	o := &Order{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		Status:     StatusPending,
		TotalPrice: 0,
		Currency:   "USD",
		Items:      []OrderItem{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	resp := ToResponse(o)

	assert.Empty(t, resp.Items)
	assert.NotNil(t, resp.Items) // should be empty slice, not nil
}

// --- OrderStatus constants ---

func TestOrderStatus_Constants(t *testing.T) {
	assert.Equal(t, OrderStatus("pending"), StatusPending)
	assert.Equal(t, OrderStatus("confirmed"), StatusConfirmed)
	assert.Equal(t, OrderStatus("shipped"), StatusShipped)
	assert.Equal(t, OrderStatus("delivered"), StatusDelivered)
	assert.Equal(t, OrderStatus("cancelled"), StatusCancelled)
}
