#!/bin/sh
# Local fixture process, never contacts a Pulumi backend or infrastructure.
case "$*" in
  *"stack ls"*) printf '[{"name":"dev","current":true}]';;
  *"stack export"*)
    if [ "$WEB_ENCRYPTED" = "true" ] && [ "$PULUMI_CONFIG_PASSPHRASE" != "web-fixture-unlock" ]; then
      printf 'incorrect passphrase' >&2
      exit 1
    fi
    cat "$WEB_STATE"
    ;;
  *"preview"*)
    if [ "$WEB_ENCRYPTED" = "true" ] && [ "$PULUMI_CONFIG_PASSPHRASE" != "web-fixture-unlock" ]; then
      printf 'incorrect passphrase' >&2
      exit 1
    fi
    cat "$WEB_PLAN"
    ;;
  *) printf 'unexpected fixture command' >&2; exit 1;;
esac
