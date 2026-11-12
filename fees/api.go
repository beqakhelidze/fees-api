package fees

import (
	"context"
	"time"

	"encore.app/fees/domain"
)

type CreateBillRequest struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

type BillResponse struct {
	Bill domain.Bill `json:"bill"`
}

type AddLineItemRequest struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	AmountMinor int64           `json:"amount_minor"`
	Currency    domain.Currency `json:"currency"`
}

//encore:api public method=POST path=/fees/bill
func (a *App) CreateBill(ctx context.Context, req *CreateBillRequest) (*BillResponse, error) {
	bill, err := a.billService.Create(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return nil, err
	}

	return &BillResponse{Bill: *bill}, nil
}

//encore:api public method=POST path=/fees/bill/:id/line-item
func (a *App) AddLineItem(ctx context.Context, id string, req *AddLineItemRequest) (*BillResponse, error) {
	bill, err := a.billService.AddLineItem(ctx, id, domain.LineItem{
		ID:          req.ID,
		Description: req.Description,
		AmountMinor: req.AmountMinor,
		Currency:    req.Currency,
	})
	if err != nil {
		return nil, err
	}

	return &BillResponse{Bill: *bill}, nil
}

//encore:api public method=POST path=/fees/bill/:id/close
func (a *App) CloseBill(ctx context.Context, id string) (*BillResponse, error) {
	bill, err := a.billService.Close(ctx, id)
	if err != nil {
		return nil, err
	}

	return &BillResponse{Bill: *bill}, nil
}

//encore:api public method=GET path=/fees/bill/:id
func (a *App) GetBill(ctx context.Context, id string) (*BillResponse, error) {
	bill, err := a.billService.Get(ctx, id)
	if err != nil {

		return nil, err
	}
	return &BillResponse{Bill: *bill}, nil
}
