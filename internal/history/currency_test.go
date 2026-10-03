package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectCurrency(t *testing.T) {
	t.Parallel()

	day := func(month time.Month, day int) time.Time {
		return time.Date(2025, month, day, 0, 0, 0, 0, time.UTC)
	}
	snap := func(version int, currency string, at time.Time) CostSnapshot {
		return CostSnapshot{Version: version, Currency: currency, Timestamp: at, TotalMonthly: float64(version)}
	}
	usdJan := snap(1, "USD", day(time.January, 15))
	usdFeb := snap(2, "USD", day(time.February, 15))
	eurMar := snap(3, "eur", day(time.March, 15))
	gbpApr := snap(4, "GBP", day(time.April, 15))

	t.Run("dominant warning", func(t *testing.T) {
		t.Parallel()
		got, err := SelectCurrency([]CostSnapshot{usdJan, usdFeb, eurMar}, CurrencyChoice{})
		require.NoError(t, err)
		assert.Equal(t, "USD", got.Currency)
		assert.Len(t, got.Snapshots, 2)
		assert.Equal(t, "USD", got.Snapshots[0].Currency)
		assert.Equal(t, "USD", got.Snapshots[1].Currency)
		assert.Contains(t, got.Warning, "Warning: Mixed currencies detected (USD: 2 snapshots, EUR: 1 snapshot).")
		assert.Contains(t, got.Warning, "Showing USD snapshots only. Use --currency EUR to view EUR snapshots.")
	})

	t.Run("currency filter", func(t *testing.T) {
		t.Parallel()
		got, err := SelectCurrency(
			[]CostSnapshot{usdJan, usdFeb, eurMar},
			CurrencyChoice{Currency: " eur ", Strict: true},
		)
		require.NoError(t, err)
		assert.Empty(t, got.Warning)
		assert.Equal(t, "EUR", got.Currency)
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, 3, got.Snapshots[0].Version)
	})

	t.Run("strict", func(t *testing.T) {
		t.Parallel()
		got, err := SelectCurrency([]CostSnapshot{usdJan, usdFeb, eurMar}, CurrencyChoice{Strict: true})
		require.ErrorIs(t, err, ErrMixedCurrencies)
		require.ErrorContains(t, err, "Mixed currencies detected across snapshots (USD, EUR).")
		require.ErrorContains(t, err, "Use --currency to filter or --no-strict to show dominant currency")
		assert.Empty(t, got.Snapshots)
	})

	t.Run("single currency", func(t *testing.T) {
		t.Parallel()
		got, err := SelectCurrency([]CostSnapshot{usdJan, usdFeb}, CurrencyChoice{})
		require.NoError(t, err)
		assert.Empty(t, got.Warning)
		assert.Equal(t, "USD", got.Currency)
		assert.Len(t, got.Snapshots, 2)
	})

	t.Run("case fold", func(t *testing.T) {
		t.Parallel()
		lower := snap(5, "usd", day(time.May, 1))
		got, err := SelectCurrency([]CostSnapshot{usdJan, lower}, CurrencyChoice{})
		require.NoError(t, err)
		assert.Empty(t, got.Warning)
		assert.Equal(t, "USD", got.Currency)
		assert.Len(t, got.Snapshots, 2)
	})

	t.Run("tie break latest", func(t *testing.T) {
		t.Parallel()
		got, err := SelectCurrency([]CostSnapshot{usdJan, eurMar}, CurrencyChoice{})
		require.NoError(t, err)
		assert.Equal(t, "EUR", got.Currency)
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, 3, got.Snapshots[0].Version)
		assert.Contains(t, got.Warning, "Showing EUR snapshots only. Use --currency USD to view USD snapshots.")
	})

	t.Run("tie break name", func(t *testing.T) {
		t.Parallel()
		sameDay := snap(6, "USD", day(time.March, 15))
		got, err := SelectCurrency([]CostSnapshot{sameDay, eurMar}, CurrencyChoice{})
		require.NoError(t, err)
		assert.Equal(t, "EUR", got.Currency)
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, "eur", got.Snapshots[0].Currency)
	})

	t.Run("empty and missing filter", func(t *testing.T) {
		t.Parallel()
		got, err := SelectCurrency(nil, CurrencyChoice{})
		require.NoError(t, err)
		assert.Empty(t, got.Snapshots)
		assert.Empty(t, got.Warning)

		missing, missErr := SelectCurrency([]CostSnapshot{usdJan}, CurrencyChoice{Currency: "GBP"})
		require.NoError(t, missErr)
		assert.Empty(t, missing.Snapshots)
		assert.Empty(t, missing.Warning)
		assert.Equal(t, "GBP", missing.Currency)
	})

	t.Run("three currencies", func(t *testing.T) {
		t.Parallel()
		got, err := SelectCurrency([]CostSnapshot{usdJan, usdFeb, eurMar, gbpApr}, CurrencyChoice{})
		require.NoError(t, err)
		assert.Equal(t, "USD", got.Currency)
		assert.Contains(
			t,
			got.Warning,
			"Use --currency EUR or GBP to view those snapshots.",
		)
	})

	t.Run("unset currency", func(t *testing.T) {
		t.Parallel()
		blank := snap(7, "  ", day(time.June, 1))
		got, err := SelectCurrency([]CostSnapshot{blank, usdJan, usdFeb}, CurrencyChoice{})
		require.NoError(t, err)
		assert.Equal(t, "USD", got.Currency)
		assert.Contains(t, got.Warning, "(unset): 1 snapshot")
		assert.NotContains(t, got.Warning, "  :")
	})
}
