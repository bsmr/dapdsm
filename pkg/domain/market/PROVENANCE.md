# Provenance

`pkg/domain/market` is a faithful port of `internal/marketbot` from
**dune-admin** (MIT, © 2026 Ryan Wilson), pinned at commit `541e43c`.
`item-data.json` is vendored from the same source/commit.

Only change from upstream: the `package` clause (`marketbot` → `market`) and an
`ItemDataPath==""` → embedded-`item-data.json` fallback in `loadCatalog`
(tool-suite block ④a). See `LICENSE.dune-admin` for the MIT license text.
