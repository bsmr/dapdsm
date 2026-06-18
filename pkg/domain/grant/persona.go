package grant

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

const (
	personaBaseAccountID = 9100001
	personaMap           = "HaggaBasin"
	personaPartitionID   = 1
	personaControllerCls = "/Game/Dune/Characters/Player/BP_DunePlayerController.BP_DunePlayerController_C"
	personaStateCls      = "/Script/DuneSandbox.DunePlayerState"
	personaPawnCls       = "/Game/Dune/Characters/Player/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C"
)

// Persona identity. HexID is the AMQP user_id (whisper sender frame); FuncomID is
// the chat m_FuncomIdFrom; CharacterName is the in-game display name. The reserved
// hex ids are pure hex so BuildErlangWhisper accepts them; account/actor ids are in
// a reserved high range so operator views can exclude them.
type personaSpec struct {
	AccountID     int64
	HexID         string // AMQP user_id (must be pure hex)
	FuncomID      string // chat m_FuncomIdFrom
	CharacterName string
}

var personas = map[string]personaSpec{
	"GM":     {AccountID: personaBaseAccountID, HexID: "DA5EBA11DA5E0001", FuncomID: "GM#0001", CharacterName: "GM"},
	"Server": {AccountID: personaBaseAccountID + 1, HexID: "DA5EBA11DA5E0002", FuncomID: "Server#0001", CharacterName: "Server"},
}

// PersonaHexID returns the AMQP user_id (hex) for a persona, or "" if unknown.
func PersonaHexID(name string) string {
	p, ok := personas[name]
	if !ok {
		return ""
	}
	return p.HexID
}

// PersonaFuncomID returns the chat funcom-id (m_FuncomIdFrom) for a persona, or "".
func PersonaFuncomID(name string) string {
	p, ok := personas[name]
	if !ok {
		return ""
	}
	return p.FuncomID
}

// PersonaName returns the in-game display name, or "".
func PersonaName(name string) string {
	p, ok := personas[name]
	if !ok {
		return ""
	}
	return p.CharacterName
}

// Persona seeds reserved sender identities into the DB so whispers can carry a real
// (non-spoofed) sender. Idempotent.
type Persona struct {
	DB    *dbquery.Runner
	Store *store.Store
}

// Seed idempotently writes the persona's base-table identity (account + 3 actors +
// player_state), mirroring dune-admin's verified GM-identity seed. Audited.
func (p *Persona) Seed(ctx context.Context, operator, host, name string) error {
	spec, ok := personas[name]
	if !ok {
		return fmt.Errorf("unknown persona %q (want GM|Server)", name)
	}
	seed := dbquery.PersonaSeed{
		AccountID:       spec.AccountID,
		ControllerID:    spec.AccountID*100 + 1,
		StateID:         spec.AccountID*100 + 2,
		PawnID:          spec.AccountID*100 + 3,
		HexID:           spec.HexID,
		FuncomID:        spec.FuncomID,
		CharacterName:   spec.CharacterName,
		ControllerClass: personaControllerCls,
		StateClass:      personaStateCls,
		PawnClass:       personaPawnCls,
		Map:             personaMap,
		PartitionID:     personaPartitionID,
		DimensionIndex:  0,
		LifeState:       "Alive",
		OnlineStatus:    "Offline",
	}
	err := p.DB.SeedPersona(ctx, host, seed)
	e := store.AuditEntry{Operator: operator, Host: host, Action: "persona.seed", Subject: "persona=" + name, Result: "ok"}
	if err != nil {
		e.Result = "error: " + err.Error()
	}
	_ = p.Store.AppendAudit(e)
	return err
}
