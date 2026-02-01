package greenops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{
			name: "small number no separators",
			n:    123,
			want: "123",
		},
		{
			name: "four digits with separator",
			n:    1234,
			want: "1,234",
		},
		{
			name: "thousands",
			n:    18248,
			want: "18,248",
		},
		{
			name: "millions",
			n:    1234567,
			want: "1,234,567",
		},
		{
			name: "zero",
			n:    0,
			want: "0",
		},
		{
			name: "negative number",
			n:    -1234,
			want: "-1,234",
		},
		{
			name: "large number",
			n:    1234567890,
			want: "1,234,567,890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatNumber(tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name      string
		f         float64
		precision int
		want      string
	}{
		{
			name:      "round to integer",
			f:         18248.56,
			precision: 0,
			want:      "18,249",
		},
		{
			name:      "one decimal place",
			f:         781.25,
			precision: 1,
			want:      "781.3",
		},
		{
			name:      "two decimal places",
			f:         1234.5678,
			precision: 2,
			want:      "1,234.57",
		},
		{
			name:      "small number",
			f:         0.5,
			precision: 1,
			want:      "0.5",
		},
		{
			name:      "zero",
			f:         0.0,
			precision: 2,
			want:      "0.00",
		},
		{
			name:      "negative with precision",
			f:         -1234.56,
			precision: 2,
			want:      "-1,234.56",
		},
		{
			name:      "round up at boundary",
			f:         999.999,
			precision: 2,
			want:      "1,000.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFloat(tt.f, tt.precision)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatLarge(t *testing.T) {
	tests := []struct {
		name string
		n    float64
		want string
	}{
		{
			name: "below threshold uses comma format",
			n:    999999,
			want: "999,999",
		},
		{
			name: "exactly one million",
			n:    1000000,
			want: "~1.0 million",
		},
		{
			name: "millions with decimal",
			n:    5200000,
			want: "~5.2 million",
		},
		{
			name: "large millions",
			n:    123400000,
			want: "~123.4 million",
		},
		{
			name: "exactly one billion",
			n:    1000000000,
			want: "~1.0 billion",
		},
		{
			name: "billions with decimal",
			n:    1500000000,
			want: "~1.5 billion",
		},
		{
			name: "small number",
			n:    1234,
			want: "1,234",
		},
		{
			name: "zero",
			n:    0,
			want: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLarge(tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Benchmarks for formatter functions

func BenchmarkFormatNumber(b *testing.B) {
	for b.Loop() {
		FormatNumber(18248)
	}
}

func BenchmarkFormatFloat(b *testing.B) {
	for b.Loop() {
		FormatFloat(1234.5678, 2)
	}
}

func BenchmarkFormatLarge(b *testing.B) {
	for b.Loop() {
		FormatLarge(52000000)
	}
}
