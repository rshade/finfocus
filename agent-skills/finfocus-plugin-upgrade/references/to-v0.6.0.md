# Upgrade to finfocus-spec v0.6.0: typed validation

v0.6.0 tightened request validation and gave its errors types.

## Manual

1. **`ValidateBatchCostRequest` returns a result struct.** Only plugins that
   implement `BatchCost` call it.

   ```go
   // Before
   early, err := pluginsdk.ValidateBatchCostRequest(req, maxBatch)
   if err != nil {
       return nil, err
   }
   if early != nil {
       return early, nil
   }

   // After
   result, err := pluginsdk.ValidateBatchCostRequest(req, maxBatch)
   if err != nil {
       return nil, err
   }
   if result.EarlyResponse != nil {
       return result.EarlyResponse, nil
   }
   ```

2. **Match validation errors by type, not text.** Validation failures are now
   `*pluginsdk.ValidationError`, and each wraps a sentinel such as
   `pluginsdk.ErrProjectedCostRegionEmpty`. Replace
   `strings.Contains(err.Error(), "...")` with `errors.Is(err, pluginsdk.Err…)`
   or `errors.As(err, &validationErr)`.

3. **Oversized descriptor fields are rejected.** `ValidateResourceDescriptor`
   enforces length and tag limits. Fixtures that use very long IDs, SKUs, or
   tag values now fail validation. Shorten the fixtures. Do not relax the
   check.

## Find what is left

```bash
grep -rn 'ValidateBatchCostRequest\|err.Error()' --include='*.go' .
```
