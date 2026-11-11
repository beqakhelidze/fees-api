package activities

import (
	"context"
	"time"

	"encore.app/fees/domain"
	"encore.app/fees/repositories"
)

// Activities are Temporal activities backed by the repository layer.
// Each one is idempotent so Temporal can safely retry it.


func New(billRepo repositories.BillRepository) *Activities {
	return &Activities{billRepo: billRepo}
}

type Activities struct {
	billRepo repositories.BillRepository
}

func (a *Activities) InsertBill(ctx context.Context, bill domain.Bill) error {
	return a.billRepo.Insert(ctx, bill)
}

func (a *Activities) InsertLineItem(ctx context.Context, billID string, item domain.LineItem) error {
	return a.billRepo.InsertLineItem(ctx, billID, item)
}

func (a *Activities) MarkBillClosed(ctx context.Context, billID string, closedAt time.Time) error {
	return a.billRepo.Close(ctx, billID, closedAt)
}
