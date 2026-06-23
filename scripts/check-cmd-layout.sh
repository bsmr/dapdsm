#!/bin/sh
# Enforce: cmd/<tool>/ must contain only main.go.
# All other logic belongs in internal/pkg/<tool>/cli/.
# Run from the dapdsm repo root.
set -e
fail=0
for d in cmd/*/; do
    extras=$(find "$d" -maxdepth 1 -name "*.go" ! -name "main.go")
    if [ -n "$extras" ]; then
        printf 'FAIL: %s must only contain main.go — move logic to internal/pkg/:\n%s\n' "$d" "$extras" >&2
        fail=1
    fi
done
exit "$fail"
