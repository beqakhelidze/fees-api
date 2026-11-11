package activities

import (
	"context"
	"fmt"
	"encore.app/fees/domain"
	"encore.app/fees/repositories"
)

// Activities are Temporal activities backed by the repository layer.
type Activities struct {
	billRepo repositories.BillRepository
}

func New(billRepo repositories.BillRepository) *Activities {
	return &Activities{billRepo: billRepo}
}

// PersistBill marks the bill closed in Postgres after the workflow finishes.
func (a *Activities) PersistBill(ctx context.Context, bill domain.Bill) error {
	if bill.ClosedAt == nil {
		return fmt.Errorf("closed_at is required to persist bill %s", bill.ID)
	}
	return a.billRepo.Close(ctx, bill.ID, *bill.ClosedAt)
}
