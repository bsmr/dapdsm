package gamedb

// vendored: see charxp_data.go

import "strings"

// xpToLevel returns the character level for the given cumulative XP (1–200).
func xpToLevel(xp int64) int {
	if xp <= 0 {
		return 0
	}
	lo, hi := 1, 200
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if cumulativeXPByLevel[mid] <= xp {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// intelAtLevel returns cumulative intel points earned through a given level.
// Based on IntelPointsRewarded curve in SkillXPPerLevel.json:
//
//	L1=4, L2-3=+2, L4-15=+3, L16-30=+5, L31-50=+10,
//	L51-69=+20, L70-85=+30, L86-125=+40, L126+=0 (cap 2779)
func intelAtLevel(level int) int64 {
	switch {
	case level <= 0:
		return 0
	case level == 1:
		return 4
	case level <= 3:
		return 4 + int64(level-1)*2
	case level <= 15:
		return 8 + int64(level-3)*3
	case level <= 30:
		return 44 + int64(level-15)*5
	case level <= 50:
		return 119 + int64(level-30)*10
	case level <= 69:
		return 319 + int64(level-50)*20
	case level <= 85:
		return 699 + int64(level-69)*30
	case level <= 125:
		return 1179 + int64(level-85)*40
	default:
		return 2779
	}
}

type charXPOutcome struct {
	newXP        int64
	newLevel     int64
	newTotalSP   int64
	newUnspentSP int64
	newIntel     int64
	capped       bool
}

func computeAwardCharXPOutcome(currentXP, spentSP, keystoneBonus, amount int64) charXPOutcome {
	newXP := currentXP + amount
	if newXP > maxCharXP {
		newXP = maxCharXP
	}
	newLevel := int64(xpToLevel(newXP))
	newTotalSP := newLevel + keystoneBonus
	// Starter job always occupies 1 SP that is excluded from spentSP.
	newUnspentSP := newTotalSP - spentSP - 1
	if newUnspentSP < 0 {
		newUnspentSP = 0
	}
	newIntel := intelAtLevel(int(newLevel))
	return charXPOutcome{
		newXP:        newXP,
		newLevel:     newLevel,
		newTotalSP:   newTotalSP,
		newUnspentSP: newUnspentSP,
		newIntel:     newIntel,
		capped:       newXP == maxCharXP,
	}
}

// keystoneSPBonus returns the total extra skill points granted by a set of keystone IDs.
func keystoneSPBonus(ids []int16) int64 {
	var total int64
	for _, id := range ids {
		info, ok := keystoneMap[id]
		if !ok {
			continue
		}
		switch {
		case strings.HasSuffix(info.Name, "_SkillPoint_Super"):
			total += 5
		case strings.HasSuffix(info.Name, "_SkillPoint_Major"):
			total += 3
		case strings.HasSuffix(info.Name, "_SkillPoint"):
			total += 1
		}
	}
	return total
}
