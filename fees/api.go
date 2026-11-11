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

//encore:api public method=POST path=/fees/bills
func (a *App) CreateBill(ctx context.Context, req *CreateBillRequest) (*BillResponse, error) {
	bill, err := a.bills.Create(ctx, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return nil, err
	}
	return &BillResponse{Bill: *bill}, nil
}

//encore:api public method=POST path=/fees/bills/:id/line-items
func (a *App) AddLineItem(ctx context.Context, id string, req *AddLineItemRequest) (*BillResponse, error) {
	bill, err := a.bills.AddLineItem(ctx, id, domain.LineItem{
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

//encore:api public method=POST path=/fees/bills/:id/close
func (a *App) CloseBill(ctx context.Context, id string) (*BillResponse, error) {
	bill, err := a.bills.Close(ctx, id)
	if err != nil {
		return nil, err
	}
	return &BillResponse{Bill: *bill}, nil
}

//encore:api public method=GET path=/fees/bills/:id
func (a *App) GetBill(ctx context.Context, id string) (*BillResponse, error) {
	bill, err := a.bills.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &BillResponse{Bill: *bill}, nil
}
