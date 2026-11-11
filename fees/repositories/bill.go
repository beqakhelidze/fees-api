package repositories

import (
	"context"
	"time"

	"encore.app/fees/domain"
	"encore.dev/storage/sqldb"
)

type BillRepository interface {
	Insert(ctx context.Context, bill domain.Bill) error
	UpsertSnapshot(ctx context.Context, bill domain.Bill) error
	GetByID(ctx context.Context, id string) (*domain.Bill, error)
}

func NewBillRepository(db *sqldb.Database) BillRepository {
	return &postgresBillRepository{db: db}
}

type postgresBillRepository struct {
	db *sqldb.Database
}

func (r *postgresBillRepository) Insert(ctx context.Context, bill domain.Bill) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO bills (id, status, period_start, period_end, closed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`, bill.ID, string(bill.Status), bill.PeriodStart, bill.PeriodEnd, bill.ClosedAt)
	return err
}

func (r *postgresBillRepository) UpsertSnapshot(ctx context.Context, bill domain.Bill) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO bills (id, status, period_start, period_end, closed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			closed_at = EXCLUDED.closed_at,
			updated_at = NOW()
	`, bill.ID, string(bill.Status), bill.PeriodStart, bill.PeriodEnd, bill.ClosedAt)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `DELETE FROM line_items WHERE bill_id = $1`, bill.ID)
	if err != nil {
		return err
	}

	for _, item := range bill.LineItems {
		_, err = r.db.Exec(ctx, `
			INSERT INTO line_items (bill_id, id, description, amount_minor, currency, created_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
		`, bill.ID, item.ID, item.Description, item.AmountMinor, string(item.Currency))
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *postgresBillRepository) GetByID(ctx context.Context, id string) (*domain.Bill, error) {
	var (
		status      string
		periodStart time.Time
		periodEnd   time.Time
		closedAt    *time.Time
	)
	err := r.db.QueryRow(ctx, `
		SELECT status, period_start, period_end, closed_at
		FROM bills WHERE id = $1
	`, id).Scan(&status, &periodStart, &periodEnd, &closedAt)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, description, amount_minor, currency
		FROM line_items WHERE bill_id = $1 ORDER BY created_at ASC, id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.LineItem{}
	for rows.Next() {
		var item domain.LineItem
		var currency string
		if err := rows.Scan(&item.ID, &item.Description, &item.AmountMinor, &currency); err != nil {
			return nil, err
		}
		item.Currency = domain.Currency(currency)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return domain.BillWithTotals(&domain.Bill{
		ID:          id,
		Status:      domain.BillStatus(status),
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		LineItems:   items,
		ClosedAt:    closedAt,
	}), nil
}
