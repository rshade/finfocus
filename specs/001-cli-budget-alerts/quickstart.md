# Quickstart: Budgeting

Set up a monthly budget and alerts in minutes.

## 1. Configure your budget
Add a `cost` section to your `~/.finfocus/config.yaml`:

```yaml
cost:
  budgets:
    amount: 1000
    currency: USD
    period: monthly
    alerts:
      - threshold: 50
        type: actual
      - threshold: 80
        type: actual
      - threshold: 100
        type: forecasted
```

## 2. Run a cost command
Run your usual cost analysis command:

```bash
finfocus cost
```

## 3. View status
In TTY mode, you will see a stylized budget status block:

```text
╭──────────────────────────────────────────╮
│ BUDGET STATUS                            │
│ ──────────────────────────────────────── │
│                                          │
│ Budget: $1,000.00/month                  │
│ Current Spend: $550.00 (55%)             │
│                                          │
│ ████████████████░░░░░░░░░░░░ 55%         │
│                                          │
│ ⚠ WARNING - Exceeds 50% threshold        │
╰──────────────────────────────────────────╯
```

In CI/CD environments, you will get a plain-text summary:

```text
BUDGET STATUS
=============
Budget: $1,000.00/month
Current Spend: $550.00 (55%)
Status: WARNING - Exceeds 50% threshold
```
