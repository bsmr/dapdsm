package dbquery

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// CharXPResult is the post-recompute summary returned by GrantCharXP.
type CharXPResult struct {
	NewLevel   int64
	NewUnspent int64
	Capped     bool
}

// GrantCharXP adds amount to the character's TotalXPEarned and recomputes level,
// TotalSkillPoints, UnspentSkillPoints, and Intel — mirroring dune-admin's
// verified flow (readCharXPState + keystoneSPBonus + computeAwardCharXPOutcome +
// applyAwardCharXPFLevelUpdate + intel). DB-only; the caller gates on offline.
func (r *Runner) GrantCharXP(ctx context.Context, host, fls string, amount int64) (CharXPResult, error) {
	const readSQL = `SELECT
  (fe.components->'FLevelComponent'->1->>'TotalXPEarned')::bigint,
  COALESCE((SELECT SUM((v->>'SkillPointsSpent')::int)
     FROM jsonb_each(fe.components->'FLevelComponent'->1->'ModuleData') AS kv(k, v)
     WHERE k != format('(TagName="%s")', fe.components->'FLevelComponent'->1->'StarterSkillTreeTag'->>'TagName')), 0),
  ps.player_pawn_id, ps.player_controller_id
FROM dune.fgl_entities fe
JOIN dune.actor_fgl_entities afe ON afe.entity_id = fe.entity_id
JOIN dune.player_state ps ON ps.player_pawn_id = afe.actor_id
JOIN dune.accounts a ON a.id = ps.account_id
WHERE afe.slot_name = 'DuneCharacter' AND a."user"::text = :'fls' ORDER BY afe.entity_id DESC LIMIT 1;`
	res, err := r.execWithVars(ctx, host, readSQL, map[string]string{"fls": fls})
	if err != nil {
		return CharXPResult{}, fmt.Errorf("char xp: read state: %w", err)
	}
	fields := strings.Split(strings.TrimSpace(res.Stdout), "|")
	if len(fields) != 4 || fields[0] == "" {
		return CharXPResult{}, fmt.Errorf("char xp: no character found (unknown fls)")
	}
	currentXP, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return CharXPResult{}, fmt.Errorf("char xp: parse currentXP %q: %w", fields[0], err)
	}
	spentSP, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return CharXPResult{}, fmt.Errorf("char xp: parse spentSP %q: %w", fields[1], err)
	}
	pawn, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return CharXPResult{}, fmt.Errorf("char xp: parse pawn %q: %w", fields[2], err)
	}
	controller, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return CharXPResult{}, fmt.Errorf("char xp: parse controller %q: %w", fields[3], err)
	}

	bonus, err := r.charXPKeystoneBonus(ctx, host, controller)
	if err != nil {
		return CharXPResult{}, err
	}

	out := computeAwardCharXPOutcome(currentXP, spentSP, bonus, amount)

	const applySQL = onErrorStop + `BEGIN;
UPDATE dune.fgl_entities
SET components = jsonb_set(jsonb_set(jsonb_set(components,
    '{FLevelComponent,1,TotalXPEarned}',     to_jsonb(:newxp::bigint)),
    '{FLevelComponent,1,TotalSkillPoints}',  to_jsonb(:newtotal::bigint)),
    '{FLevelComponent,1,UnspentSkillPoints}', to_jsonb(:newunspent::bigint))
WHERE entity_id = (SELECT entity_id FROM dune.actor_fgl_entities
                   WHERE actor_id = :pawn::bigint AND slot_name = 'DuneCharacter');
UPDATE dune.actors
SET properties = jsonb_set(properties,
    '{TechKnowledgePlayerComponent,m_TechKnowledgePoints}', to_jsonb(:newintel::bigint))
WHERE id = :pawn::bigint AND properties ? 'TechKnowledgePlayerComponent';
COMMIT;`
	_, err = r.execWithVars(ctx, host, applySQL, map[string]string{
		"newxp":      strconv.FormatInt(out.newXP, 10),
		"newtotal":   strconv.FormatInt(out.newTotalSP, 10),
		"newunspent": strconv.FormatInt(out.newUnspentSP, 10),
		"newintel":   strconv.FormatInt(out.newIntel, 10),
		"pawn":       strconv.FormatInt(pawn, 10),
	})
	if err != nil {
		return CharXPResult{}, fmt.Errorf("char xp: apply: %w", err)
	}
	return CharXPResult{NewLevel: out.newLevel, NewUnspent: out.newUnspentSP, Capped: out.capped}, nil
}

// charXPKeystoneBonus reads the controller's purchased keystone ids and maps them
// to bonus skill points via the vendored keystoneSPBonus.
func (r *Runner) charXPKeystoneBonus(ctx context.Context, host string, controller int64) (int64, error) {
	if controller == 0 {
		return 0, nil
	}
	const sql = `SELECT keystone_id FROM dune.purchased_specialization_keystones WHERE player_id = :ctrl::bigint;`
	res, err := r.execWithVars(ctx, host, sql, map[string]string{"ctrl": strconv.FormatInt(controller, 10)})
	if err != nil {
		return 0, fmt.Errorf("char xp: read keystones: %w", err)
	}
	var ids []int16
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.ParseInt(line, 10, 16)
		if err != nil {
			return 0, fmt.Errorf("char xp: parse keystone id %q: %w", line, err)
		}
		ids = append(ids, int16(v))
	}
	return keystoneSPBonus(ids), nil
}
