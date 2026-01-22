package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func TestNewRecommendationRow(t *testing.T) {
	t.Run("basic recommendation", func(t *testing.T) {
		rec := engine.Recommendation{
			ResourceID:       "aws:ec2:Instance/i-123",
			Type:             "RIGHTSIZE",
			Description:      "Downsize from m5.xlarge to m5.large",
			EstimatedSavings: 87.60,
			Currency:         "USD",
		}

		row := NewRecommendationRow(rec)

		assert.Equal(t, "aws:ec2:Instance/i-123", row.ResourceID)
		assert.Equal(t, "RIGHTSIZE", row.ActionType)
		assert.Equal(t, "Downsize from m5.xlarge to m5.large", row.Description)
		assert.Equal(t, "$87.60 USD", row.Savings)
		assert.True(t, row.HasSavings)
	})

	t.Run("zero savings", func(t *testing.T) {
		rec := engine.Recommendation{
			ResourceID:       "aws:s3:Bucket/my-bucket",
			Type:             "MODIFY",
			Description:      "Enable intelligent tiering",
			EstimatedSavings: 0,
			Currency:         "USD",
		}

		row := NewRecommendationRow(rec)

		assert.Equal(t, "$0.00 USD", row.Savings)
		assert.False(t, row.HasSavings)
	})

	t.Run("long description truncation", func(t *testing.T) {
		longDesc := "This is a very long description that exceeds forty characters and should be truncated"
		rec := engine.Recommendation{
			ResourceID:       "res-1",
			Type:             "RIGHTSIZE",
			Description:      longDesc,
			EstimatedSavings: 50.00,
			Currency:         "USD",
		}

		row := NewRecommendationRow(rec)

		// Description should be truncated to maxDescLen with "..."
		assert.LessOrEqual(t, len(row.Description), maxDescLen)
		if len(longDesc) > maxDescLen {
			assert.True(t, len(row.Description) <= maxDescLen)
			assert.Contains(t, row.Description, "...")
		}
	})

	t.Run("long resource ID truncation", func(t *testing.T) {
		longResourceID := "aws:ec2:Instance/i-0123456789abcdef0123456789abcdef"
		rec := engine.Recommendation{
			ResourceID:       longResourceID,
			Type:             "TERMINATE",
			Description:      "Terminate idle instance",
			EstimatedSavings: 100.00,
			Currency:         "USD",
		}

		row := NewRecommendationRow(rec)

		// Resource ID should be truncated to maxResourceIDLen with "..."
		assert.LessOrEqual(t, len(row.ResourceID), maxResourceIDLen)
	})

	t.Run("empty currency defaults to USD", func(t *testing.T) {
		rec := engine.Recommendation{
			ResourceID:       "res-1",
			Type:             "RIGHTSIZE",
			Description:      "Test",
			EstimatedSavings: 50.00,
			Currency:         "",
		}

		row := NewRecommendationRow(rec)

		assert.Contains(t, row.Savings, "USD")
	})

	t.Run("different currencies", func(t *testing.T) {
		rec := engine.Recommendation{
			ResourceID:       "res-1",
			Type:             "RIGHTSIZE",
			Description:      "Test",
			EstimatedSavings: 50.00,
			Currency:         "EUR",
		}

		row := NewRecommendationRow(rec)

		assert.Equal(t, "$50.00 EUR", row.Savings)
	})
}

func TestRecommendationRowConstants(t *testing.T) {
	t.Run("constants are reasonable", func(t *testing.T) {
		// Verify constants are set to reasonable values
		assert.GreaterOrEqual(t, maxDescLen, 30)
		assert.LessOrEqual(t, maxDescLen, 60)
		assert.GreaterOrEqual(t, maxResourceIDLen, 25)
		assert.LessOrEqual(t, maxResourceIDLen, 50)
	})
}
