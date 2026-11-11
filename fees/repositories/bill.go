package repositories

import (
	"context"
	"time"

	"encore.app/fees/domain"
	"encore.dev/storage/sqldb"
)

type BillRepository interface {
	Insert(ctx context.Context, bill domain.Bill) error
	InsertLineItem(ctx context.Context, billID string, item domain.LineItem) error
	Close(ctx context.Context, billID string, closedAt time.Time) error
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
		ON CONFLICT (id) DO NOTHING
	`, bill.ID, string(bill.Status), bill.PeriodStart, bill.PeriodEnd, bill.ClosedAt)
	return err
}

func (r *postgresBillRepository) InsertLineItem(ctx context.Context, billID string, item domain.LineItem) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO line_items (bill_id, id, description, amount_minor, currency, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (bill_id, id) DO NOTHING
	`, billID, item.ID, item.Description, item.AmountMinor, string(item.Currency))
	return err
}

func (r *postgresBillRepository) Close(ctx context.Context, billID string, closedAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE bills
		SET status = $2, closed_at = $3, updated_at = NOW()
		WHERE id = $1
	`, billID, string(domain.BillStatusClosed), closedAt)
	return err
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

	return &domain.Bill{
		ID:          id,
		Status:      domain.BillStatus(status),
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		LineItems:   items,
		ClosedAt:    closedAt,
	}, nil
}
