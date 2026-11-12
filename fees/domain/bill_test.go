package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTotalsFromLineItems(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		require.Empty(t, TotalsFromLineItems(nil))
		require.Empty(t, TotalsFromLineItems([]LineItem{}))
	})

	t.Run("sums per currency and keeps first-seen order", func(t *testing.T) {
		totals := TotalsFromLineItems([]LineItem{
			{ID: "1", AmountMinor: 200, Currency: "gel"},
			{ID: "2", AmountMinor: 150, Currency: CurrencyUSD},
			{ID: "3", AmountMinor: 50, Currency: CurrencyGEL},
		})
		require.Equal(t, []Money{
			{AmountMinor: 250, Currency: CurrencyGEL},
			{AmountMinor: 150, Currency: CurrencyUSD},
		}, totals)
	})
}

func TestNormalizeCurrency(t *testing.T) {
	require.Equal(t, CurrencyUSD, NormalizeCurrency("usd"))
	require.Equal(t, CurrencyGEL, NormalizeCurrency(CurrencyGEL))
}
