#!/bin/sh
# Local fixture process, never contacts a Pulumi backend or infrastructure.
# WEB_STACKS lists the project's stacks (comma separated). WEB_STATE_DIR holds an
# optional <stack>.json state per stack, and WEB_FAIL_STACK names a stack whose
# state cannot be read.
stack=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--stack" ]; then stack="$arg"; fi
  prev="$arg"
done
case "$*" in
  *"stack ls"*)
    if [ -n "$WEB_STACKS" ]; then
      out=""
      for name in $(printf '%s' "$WEB_STACKS" | tr ',' ' '); do
        out="$out${out:+,}{\"name\":\"$name\",\"current\":false}"
      done
      printf '[%s]' "$out"
    else
      printf '[{"name":"dev","current":true}]'
    fi
    ;;
  *"stack export"*)
    if [ -n "$stack" ] && [ "$stack" = "$WEB_FAIL_STACK" ]; then
      printf 'fixture failure for %s' "$stack" >&2
      exit 1
    fi
    if [ "$WEB_ENCRYPTED" = "true" ] && [ "$PULUMI_CONFIG_PASSPHRASE" != "web-fixture-unlock" ]; then
      printf 'incorrect passphrase' >&2
      exit 1
    fi
    if [ -n "$stack" ] && [ -n "$WEB_STATE_DIR" ] && [ -f "$WEB_STATE_DIR/$stack.json" ]; then
      cat "$WEB_STATE_DIR/$stack.json"
    else
      cat "$WEB_STATE"
    fi
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
