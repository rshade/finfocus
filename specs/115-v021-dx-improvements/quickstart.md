# Quickstart: v0.2.1 DX Improvements

## Fast Plugin Listing

The `plugin list` command is now significantly faster, fetching plugin details in parallel.

```bash
finfocus plugin list
```

**Output:**
```text
Name        Version   Spec Version   Path
----        -------   ------------   ----
aws-public  v0.0.7    v0.4.14        /home/user/.finfocus/plugins/aws-public/v0.0.7/...
azure-cost  v0.1.2    Legacy         /home/user/.finfocus/plugins/azure-cost/v0.1.2/...
```

- **Spec Version**: Shows the protocol version the plugin supports.
- **Legacy**: Indicates the plugin is older and doesn't report its spec version (but may still work).

## Cleaner Plugin Upgrades

Automatically remove old versions when installing a plugin to save disk space.

```bash
# Install latest version and remove all older versions of 'aws-public'
finfocus plugin install aws-public --clean
```

## Consistent Filtering

Filter flags now behave identically across `cost actual` and `cost projected`.

```bash
# Filter by region
finfocus cost actual --filter "region=us-east-1"

# Same syntax works for projected costs
finfocus cost projected --filter "region=us-east-1"
```
