package domain

import (
	"strings"
	"time"
)

type BillStatus string

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyGEL Currency = "GEL"
)

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

type Money struct {
	AmountMinor int64    `json:"amount_minor"`
	Currency    Currency `json:"currency"`
}

type Bill struct {
	ID          string     `json:"id"`
	Status      BillStatus `json:"status"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	LineItems   []LineItem `json:"line_items"`
	Totals      []Money    `json:"totals,omitempty"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

func TotalsFromLineItems(items []LineItem) []Money {
	sums := map[Currency]int64{}
	order := []Currency{}

	for _, item := range items {
		c := NormalizeCurrency(item.Currency)
		if _, ok := sums[c]; !ok {
			order = append(order, c)
		}
		sums[c] += item.AmountMinor
	}

	out := make([]Money, 0, len(order))
	for _, c := range order {
		out = append(out, Money{AmountMinor: sums[c], Currency: c})
	}

	return out
}

func NormalizeCurrency(c Currency) Currency {
	return Currency(strings.ToUpper(string(c)))
}

func BillWithTotals(bill *Bill) *Bill {
	if bill == nil {
		return nil
	}

	bill.Totals = TotalsFromLineItems(bill.LineItems)
	return bill
}
