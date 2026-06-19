// Package market is the NPC market engine for ds-spice (tool-suite block ④).
// It is a faithful port of dune-admin's internal/marketbot (MIT) — see
// PROVENANCE.md. The reusable core is Run(ctx, BotConfig) (*Instance, error);
// ④b wraps it in the cmd/ds-spice daemon fed by pkg/config.
package market
