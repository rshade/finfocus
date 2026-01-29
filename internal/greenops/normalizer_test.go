package greenops

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeToKg(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		unit    string
		wantKg  float64
		wantErr bool
		errType error
	}{
		// Standard units
		{
			name:   "grams to kg",
			value:  1000.0,
			unit:   "g",
			wantKg: 1.0,
		},
		{
			name:   "kilograms identity",
			value:  150.0,
			unit:   "kg",
			wantKg: 150.0,
		},
		{
			name:   "metric tons to kg",
			value:  1.0,
			unit:   "t",
			wantKg: 1000.0,
		},
		{
			name:   "pounds to kg",
			value:  100.0,
			unit:   "lb",
			wantKg: 45.3592, // 100 * 0.453592
		},
		// CO2e suffix variants
		{
			name:   "gCO2e to kg",
			value:  150000.0,
			unit:   "gCO2e",
			wantKg: 150.0,
		},
		{
			name:   "kgCO2e identity",
			value:  150.0,
			unit:   "kgCO2e",
			wantKg: 150.0,
		},
		{
			name:   "tCO2e to kg",
			value:  0.15,
			unit:   "tCO2e",
			wantKg: 150.0,
		},
		{
			name:   "lbCO2e to kg",
			value:  330.69,
			unit:   "lbCO2e",
			wantKg: 150.0, // Approximately (330.69 * 0.453592)
		},
		// Case insensitivity
		{
			name:   "uppercase KG",
			value:  100.0,
			unit:   "KG",
			wantKg: 100.0,
		},
		{
			name:   "mixed case Kg",
			value:  100.0,
			unit:   "Kg",
			wantKg: 100.0,
		},
		// Edge cases
		{
			name:   "zero value",
			value:  0.0,
			unit:   "kg",
			wantKg: 0.0,
		},
		{
			name:   "very small value",
			value:  0.001,
			unit:   "g",
			wantKg: 0.000001,
		},
		{
			name:   "very large value",
			value:  1000000.0,
			unit:   "t",
			wantKg: 1000000000.0, // 1 billion kg
		},
		// Error cases
		{
			name:    "invalid unit",
			value:   100.0,
			unit:    "invalid",
			wantErr: true,
			errType: ErrInvalidUnit,
		},
		{
			name:    "empty unit",
			value:   100.0,
			unit:    "",
			wantErr: true,
			errType: ErrInvalidUnit,
		},
		{
			name:    "negative value",
			value:   -100.0,
			unit:    "kg",
			wantErr: true,
			errType: ErrNegativeValue,
		},
		// Overflow cases
		{
			name:    "positive infinity",
			value:   math.Inf(1),
			unit:    "kg",
			wantErr: true,
			errType: ErrCalculationOverflow,
		},
		{
			name:    "negative infinity",
			value:   math.Inf(-1),
			unit:    "kg",
			wantErr: true,
			errType: ErrCalculationOverflow,
		},
		{
			name:    "NaN value",
			value:   math.NaN(),
			unit:    "kg",
			wantErr: true,
			errType: ErrCalculationOverflow,
		},
		// Multiplication overflow case
		{
			name:    "multiplication overflow",
			value:   math.MaxFloat64 / 100, // Large enough that *1000 overflows to Inf
			unit:    "t",                   // factor = 1000
			wantErr: true,
			errType: ErrCalculationOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeToKg(tt.value, tt.unit)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				return
			}

			require.NoError(t, err)
			// Use InDelta for floating point comparison (0.01% tolerance)
			assert.InDelta(t, tt.wantKg, got, tt.wantKg*0.0001+0.0001)
		})
	}
}

func TestIsRecognizedUnit(t *testing.T) {
	tests := []struct {
		unit string
		want bool
	}{
		// Valid units
		{"g", true},
		{"kg", true},
		{"t", true},
		{"lb", true},
		{"gCO2e", true},
		{"kgCO2e", true},
		{"tCO2e", true},
		{"lbCO2e", true},
		// Case insensitivity
		{"G", true},
		{"KG", true},
		{"Kg", true},
		{"TCO2E", true},
		// Invalid units
		{"", false},
		{"invalid", false},
		{"oz", false},
		{"ton", false}, // We use 't' not 'ton'
	}

	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			got := IsRecognizedUnit(tt.unit)
			assert.Equal(t, tt.want, got)
		})
	}
}
