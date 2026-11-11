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
