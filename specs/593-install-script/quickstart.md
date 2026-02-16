# Quickstart: Install Script

## Install FinFocus

```bash
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

## Verify

```bash
finfocus --version
```

## Options

### Install a specific version

```bash
FINFOCUS_VERSION=v0.2.0 curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

### Install to a custom directory

```bash
FINFOCUS_INSTALL_DIR=$HOME/bin curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

### Skip checksum verification (not recommended)

```bash
FINFOCUS_NO_VERIFY=1 curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

## Alternative: wget

```bash
wget -qO- https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

## Uninstall

```bash
rm -f /usr/local/bin/finfocus
# Or if installed to user-local:
rm -f $HOME/.local/bin/finfocus
```
