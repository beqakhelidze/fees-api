package domain

import "time"

type BillStatus string

const (
	BillStatusOpen   BillStatus = "OPEN"
	BillStatusClosed BillStatus = "CLOSED"
)

type LineItem struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	AmountMinor int64    `json:"amount_minor"`
	Currency    Currency `json:"currency"`
}

type Bill struct {
	ID          string     `json:"id"`
	Status      BillStatus `json:"status"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	LineItems   []LineItem `json:"line_items"`
	Totals      []Money    `json:"totals"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}
