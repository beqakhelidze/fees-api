package domain

import (
	"strings"

	"encore.dev/beta/errs"
)

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyGEL Currency = "GEL"
)

type Money struct {
	AmountMinor int64    `json:"amount_minor"`
	Currency    Currency `json:"currency"`
}

func ValidateCurrency(c Currency) error {
	switch Currency(strings.ToUpper(string(c))) {
	case CurrencyUSD, CurrencyGEL:
		return nil
	default:
		return errs.B().Code(errs.InvalidArgument).Msg("currency must be USD or GEL").Err()
	}
}

func NormalizeCurrency(c Currency) Currency {
	return Currency(strings.ToUpper(string(c)))
}

func ValidateMoney(m Money) error {
	if err := ValidateCurrency(m.Currency); err != nil {
		return err
	}
	if m.AmountMinor <= 0 {
		return errs.B().Code(errs.InvalidArgument).Msg("amount_minor must be positive").Err()
	}
	return nil
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
