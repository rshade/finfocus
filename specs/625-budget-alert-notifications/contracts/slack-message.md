# Contract: Slack Incoming-Webhook Message

`POST <url>` with `Content-Type: application/json`. Success is any 2xx.

```json
{
  "channel": "#finops-alerts",
  "text": "Budget alert: global budget crossed its 80% actual threshold",
  "blocks": [
    {"type": "header", "text": {"type": "plain_text", "text": "Budget alert"}},
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*Budget:*\nglobal, $100.00/month"},
        {"type": "mrkdwn", "text": "*Current spend:*\n$85.00 (85%)"},
        {"type": "mrkdwn", "text": "*Threshold:*\n80% actual"},
        {"type": "mrkdwn", "text": "*Status:*\nExceeded"}
      ]
    }
  ]
}
```

- `channel` is present only when configured.
- For a forecasted alert the spend field is labeled `*Forecasted spend:*`
  and the `text` says `forecasted`.
- Scoped budgets name the scope, for example `provider aws` or
  `tag team:platform`.
- Amounts use the budget currency symbol and two decimals; percentages are
  whole numbers when they are integral, otherwise one decimal.
