package forecast

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestParseSpendCSV_Daily(t *testing.T) {
	t.Parallel()
	got, err := ParseSpendCSV(
		strings.NewReader("date,value\n2026-04-02,51.8\n2026-04-01,52.10\n2026-04-03T18:30:00-04:00,50\n"),
	)
	require.NoError(t, err)
	assert.Equal(t, CadenceDaily, got.Cadence)
	assert.Equal(t, []Point{
		{Time: day(2026, 4, 1), Value: 52.10},
		{Time: day(2026, 4, 2), Value: 51.8},
		{Time: day(2026, 4, 3), Value: 50},
	}, got.Points, "rows are sorted and an RFC3339 time keeps its UTC day")
}

func TestParseSpendCSV_MonthlyNormalizesToMonthStart(t *testing.T) {
	t.Parallel()
	got, err := ParseSpendCSV(strings.NewReader("2026-01-31,1000\n2026-02-28,1100\n2026-03-31,1200\n"))
	require.NoError(t, err)
	assert.Equal(t, CadenceMonthly, got.Cadence)
	assert.Equal(t, []Point{
		{Time: day(2026, 1, 1), Value: 1000},
		{Time: day(2026, 2, 1), Value: 1100},
		{Time: day(2026, 3, 1), Value: 1200},
	}, got.Points)
}

func TestParseSpendCSV_SingleRowIsDaily(t *testing.T) {
	t.Parallel()
	got, err := ParseSpendCSV(strings.NewReader("2026-04-01,5\n"))
	require.NoError(t, err)
	assert.Equal(t, CadenceDaily, got.Cadence)
	require.Len(t, got.Points, 1)
}

func TestParseSpendCSV_Errors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		csv  string
		want string
	}{
		{name: "empty", csv: "", want: "spend history has no rows"},
		{name: "header only", csv: "date,value\n", want: "spend history has no rows"},
		{name: "one column", csv: "2026-04-01,1\n2026-04-02\n",
			want: "spend history line 2: expected 2 columns (date,value), got 1"},
		{name: "three columns", csv: "2026-04-01,1,x\n",
			want: "spend history line 1: expected 2 columns (date,value), got 3"},
		{name: "bad date", csv: "2026-04-01,1\n04/02/2026,1\n",
			want: `spend history line 2: invalid date "04/02/2026" (use 2006-01-02 or RFC3339)`},
		{name: "bad value", csv: "date,value\n2026-04-01,1\n2026-04-02,ten\n",
			want: `spend history line 3: invalid value "ten"`},
		{name: "infinite value", csv: "2026-04-01,Inf\n", want: `spend history line 1: invalid value "Inf"`},
		{name: "negative", csv: "2026-04-01,-3\n", want: "spend history line 1: value must not be negative"},
		{name: "duplicate", csv: "2026-04-01,1\n2026-04-02,1\n2026-04-01,2\n",
			want: "spend history line 3: duplicate date 2026-04-01"},
		{name: "daily gap", csv: "2026-04-01,1\n2026-04-02,1\n2026-04-04,1\n",
			want: "spend history line 3: 2026-04-04 does not follow 2026-04-02; rows must be consecutive days"},
		{name: "monthly gap", csv: "2026-01-01,1\n2026-02-01,1\n2026-04-01,1\n",
			want: "spend history line 3: 2026-04-01 does not follow 2026-02-01; rows must be consecutive months"},
		{name: "two rows in a month", csv: "2026-01-01,1\n2026-02-01,1\n2026-02-15,1\n",
			want: "spend history line 3: 2026-02-15 does not follow 2026-02-01; rows must be consecutive months"},
		{name: "unterminated quote", csv: "date,value\n2026-01-01,1\n2026-01-02,2\n\"x",
			want: `spend history line 4: extraneous or missing " in quoted-field`},
		{name: "bad first data row", csv: "2026-01-01,abc\n2026-01-02,2\n",
			want: `spend history line 1: invalid value "abc"`},
		{
			name: "neither cadence",
			csv:  "2026-01-01,1\n2026-01-10,1\n",
			want: "spend history line 2: 2026-01-10 does not follow 2026-01-01; rows must be consecutive days or months",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseSpendCSV(strings.NewReader(tc.csv))
			require.EqualError(t, err, tc.want)
			assert.True(t, IsInputError(err))
		})
	}
}

func TestParseSpendCSV_ByteOrderMark(t *testing.T) {
	t.Parallel()
	got, err := ParseSpendCSV(strings.NewReader("\uFEFF2026-04-01,5\n2026-04-02,6\n"))
	require.NoError(t, err)
	assert.Len(t, got.Points, 2)
	withHeader, err := ParseSpendCSV(strings.NewReader("\uFEFFdate,value\n2026-04-01,5\n"))
	require.NoError(t, err)
	assert.Len(t, withHeader.Points, 1)
}
