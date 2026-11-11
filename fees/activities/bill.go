package activities

import (
	"context"

	"encore.app/fees/domain"
	"encore.app/fees/repository"
)

// Activities are Temporal activities backed by the repository layer.
type Activities struct {
	billRepo repository.BillRepository
}

func New(billRepo repository.BillRepository) *Activities {
	return &Activities{billRepo: billRepo}
}

// PersistBill writes the final (or current) bill snapshot to Postgres.
func (a *Activities) PersistBill(ctx context.Context, bill domain.Bill) error {
	return a.billRepo.UpsertSnapshot(ctx, bill)
}
