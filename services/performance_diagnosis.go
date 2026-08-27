package services

import (
	"fmt"
	"strings"

	"afrigoals.com/models"
)

// performanceFinding is one strength or weakness in the Team Performance
// Diagnosis section. Every finding must be traceable to real evidence -
// EventIDs are the actual contributing AnalysisEvent rows, never invented.
type performanceFinding struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // "strength" | "weakness"
	Title      string `json:"title"`
	MetricKey  string `json:"metric_key"`
	Evidence   string `json:"evidence"`
	EventIDs   []uint `json:"event_ids"`
	ClipIDs    []uint `json:"clip_ids"`
	Confidence string `json:"confidence"` // "high" | "medium" | "low"
	SampleSize int    `json:"sample_size"`
}

type teamMetricsBundle struct {
	ClubID     uint
	Events     []models.AnalysisEvent
	Attacking  metricSection
	Defensive  metricSection
	Transition metricSection
}

type diagnosisRule func(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding

// diagnoseTeam evaluates every rule below for `own` relative to `opponent`
// in the same match. Every rule requires two independent conditions to
// fire: a comparative primary signal (own vs opponent, past a minimum
// sample floor) AND a corroborating second signal (a second metric, or a
// zone/outcome cross-check on the same events). If the primary signal
// clears but corroboration disagrees, the rule returns nil - silence, not a
// low-confidence guess. This is the concrete mechanism behind "never
// conclude from a single metric alone."
func diagnoseTeam(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) (strengths, weaknesses []performanceFinding) {
	strengths = []performanceFinding{}
	weaknesses = []performanceFinding{}
	rules := []diagnosisRule{
		ruleShotAccuracyGap,
		ruleWastefulFinishing,
		ruleTurnoverRisk,
		rulePressEffectiveness,
		ruleSetPieceThreat,
		ruleDefensiveErrorRisk,
		ruleDisciplineRisk,
		ruleDuelDominance,
	}
	for _, rule := range rules {
		finding := rule(own, opponent, clipEventIDs)
		if finding == nil {
			continue
		}
		if finding.Type == "strength" {
			strengths = append(strengths, *finding)
		} else {
			weaknesses = append(weaknesses, *finding)
		}
	}
	return strengths, weaknesses
}

func findingID(slug string, clubID uint) string {
	return fmt.Sprintf("%s-%d", slug, clubID)
}

func clipIDsFor(ids []uint, clipEventIDs map[uint]bool) []uint {
	out := []uint{}
	for _, id := range ids {
		if clipEventIDs[id] {
			out = append(out, id)
		}
	}
	return out
}

func zoneIndicatesDefensiveOrMiddleThird(zone *string) bool {
	if zone == nil {
		return false
	}
	z := strings.ToLower(strings.TrimSpace(*zone))
	return strings.HasPrefix(z, "def-") || strings.HasPrefix(z, "mid-")
}

func zoneIndicatesDefensiveThird(zone *string) bool {
	if zone == nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(*zone)), "def-")
}

// ruleShotAccuracyGap: bidirectional. Primary = shot_accuracy_pct gap vs
// opponent (min 5 shots each side). Corroboration = big_chances gap agreeing
// with the direction of the accuracy gap.
func ruleShotAccuracyGap(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownAcc, okOwn := metricByKey(own.Attacking, "shot_accuracy_pct")
	oppAcc, okOpp := metricByKey(opponent.Attacking, "shot_accuracy_pct")
	ownShots, _ := metricByKey(own.Attacking, "shots_total")
	oppShots, _ := metricByKey(opponent.Attacking, "shots_total")
	ownBC, _ := metricByKey(own.Attacking, "big_chances")
	oppBC, _ := metricByKey(opponent.Attacking, "big_chances")

	if !okOwn || !okOpp || ownAcc.Value == nil || oppAcc.Value == nil {
		return nil
	}
	if ownShots.SampleSize < 5 || oppShots.SampleSize < 5 {
		return nil
	}

	gap := *ownAcc.Value - *oppAcc.Value
	bcGap := valueOrZero(ownBC.Value) - valueOrZero(oppBC.Value)

	if gap <= -15 && bcGap <= 0 {
		confidence := "medium"
		if gap <= -25 {
			confidence = "high"
		}
		return &performanceFinding{
			ID: findingID("weak-shot-accuracy", own.ClubID), Type: "weakness",
			Title:     "Shot accuracy below the opponent's",
			MetricKey: "shot_accuracy_pct",
			Evidence: fmt.Sprintf("Shot accuracy %.0f%% vs opponent's %.0f%% (%d shots vs %d), with %.0f big chances vs opponent's %.0f.",
				*ownAcc.Value, *oppAcc.Value, ownShots.SampleSize, oppShots.SampleSize, valueOrZero(ownBC.Value), valueOrZero(oppBC.Value)),
			EventIDs: ownAcc.EventIDs, ClipIDs: clipIDsFor(ownAcc.EventIDs, clipEventIDs),
			Confidence: confidence, SampleSize: ownShots.SampleSize,
		}
	}
	if gap >= 15 && bcGap >= 0 {
		confidence := "medium"
		if gap >= 25 {
			confidence = "high"
		}
		return &performanceFinding{
			ID: findingID("strong-shot-accuracy", own.ClubID), Type: "strength",
			Title:     "Shot accuracy ahead of the opponent's",
			MetricKey: "shot_accuracy_pct",
			Evidence: fmt.Sprintf("Shot accuracy %.0f%% vs opponent's %.0f%% (%d shots vs %d), with %.0f big chances vs opponent's %.0f.",
				*ownAcc.Value, *oppAcc.Value, ownShots.SampleSize, oppShots.SampleSize, valueOrZero(ownBC.Value), valueOrZero(oppBC.Value)),
			EventIDs: ownAcc.EventIDs, ClipIDs: clipIDsFor(ownAcc.EventIDs, clipEventIDs),
			Confidence: confidence, SampleSize: ownShots.SampleSize,
		}
	}
	return nil
}

// ruleWastefulFinishing: weakness-only. Primary = created at least as many
// big chances as the opponent. Corroboration = still scored fewer goals -
// the divergence between the two independently-tagged metrics is itself the
// corroborating signal.
func ruleWastefulFinishing(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownBC, _ := metricByKey(own.Attacking, "big_chances")
	oppBC, _ := metricByKey(opponent.Attacking, "big_chances")
	ownGoals, _ := metricByKey(own.Attacking, "goals")
	oppGoals, _ := metricByKey(opponent.Attacking, "goals")

	if ownBC.SampleSize < 3 {
		return nil
	}
	if valueOrZero(ownBC.Value) < valueOrZero(oppBC.Value) {
		return nil
	}
	goalGap := valueOrZero(oppGoals.Value) - valueOrZero(ownGoals.Value)
	if goalGap < 1 {
		return nil
	}
	confidence := "medium"
	if goalGap >= 2 {
		confidence = "high"
	}
	return &performanceFinding{
		ID: findingID("wasteful-finishing", own.ClubID), Type: "weakness",
		Title:     "Created enough to score more than they did",
		MetricKey: "big_chances",
		Evidence: fmt.Sprintf("%.0f big chances (vs opponent's %.0f) but only %.0f goals (opponent scored %.0f).",
			valueOrZero(ownBC.Value), valueOrZero(oppBC.Value), valueOrZero(ownGoals.Value), valueOrZero(oppGoals.Value)),
		EventIDs: ownBC.EventIDs, ClipIDs: clipIDsFor(ownBC.EventIDs, clipEventIDs),
		Confidence: confidence, SampleSize: ownBC.SampleSize,
	}
}

// ruleTurnoverRisk: weakness-only. Primary = dangerous_turnovers rate vs
// opponent (>=1.5x, min sample 3). Corroboration = at least 2 of those
// events zone-tagged in the defensive/middle third.
func ruleTurnoverRisk(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownDT, _ := metricByKey(own.Transition, "dangerous_turnovers")
	oppDT, _ := metricByKey(opponent.Transition, "dangerous_turnovers")
	if ownDT.SampleSize < 3 {
		return nil
	}
	ownCount := valueOrZero(ownDT.Value)
	oppCount := valueOrZero(oppDT.Value)
	comparisonBase := oppCount
	if comparisonBase == 0 {
		comparisonBase = 0.5 // avoid divide-by-zero while still requiring a real gap
	}
	ratio := ownCount / comparisonBase
	if ratio < 1.5 {
		return nil
	}

	turnoverEvents := filterByType(own.Events, "dangerous_turnover")
	riskyZoneCount := 0
	for _, e := range turnoverEvents {
		if zoneIndicatesDefensiveOrMiddleThird(e.PitchZone) {
			riskyZoneCount++
		}
	}
	if riskyZoneCount < 2 {
		return nil
	}

	confidence := "medium"
	if ratio >= 2 {
		confidence = "high"
	}
	ids := eventIDs(turnoverEvents)
	return &performanceFinding{
		ID: findingID("turnover-risk", own.ClubID), Type: "weakness",
		Title:     "Dangerous turnovers in risky areas",
		MetricKey: "dangerous_turnovers",
		Evidence: fmt.Sprintf("%.0f dangerous turnovers (vs opponent's %.0f), %d of them in the defensive/middle third.",
			ownCount, oppCount, riskyZoneCount),
		EventIDs: ids, ClipIDs: clipIDsFor(ids, clipEventIDs),
		Confidence: confidence, SampleSize: ownDT.SampleSize,
	}
}

// rulePressEffectiveness: bidirectional. Primary = press_success_rate gap vs
// opponent (min combined pressing sample 5). Corroboration = ball-regain
// volume (recoveries_after_loss + possession_won) agreeing with the
// direction of the rate gap.
func rulePressEffectiveness(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownRate, _ := metricByKey(own.Transition, "press_success_rate")
	oppRate, _ := metricByKey(opponent.Transition, "press_success_rate")
	ownPressing, _ := metricByKey(own.Transition, "pressing_actions")
	ownPressSuccess, _ := metricByKey(own.Transition, "press_successes")
	if ownPressing.SampleSize+ownPressSuccess.SampleSize < 5 || ownRate.Value == nil || oppRate.Value == nil {
		return nil
	}

	ownRecoveries, _ := metricByKey(own.Transition, "recoveries_after_loss")
	ownPossWon, _ := metricByKey(own.Defensive, "possession_won")
	oppRecoveries, _ := metricByKey(opponent.Transition, "recoveries_after_loss")
	oppPossWon, _ := metricByKey(opponent.Defensive, "possession_won")
	ownRegain := valueOrZero(ownRecoveries.Value) + valueOrZero(ownPossWon.Value)
	oppRegain := valueOrZero(oppRecoveries.Value) + valueOrZero(oppPossWon.Value)

	gap := *ownRate.Value - *oppRate.Value
	contributing := append(append([]models.AnalysisEvent{}, filterByType(own.Events, "press_success")...), filterByType(own.Events, "pressing_action")...)
	ids := eventIDs(contributing)
	sampleSize := ownPressing.SampleSize + ownPressSuccess.SampleSize

	if gap >= 15 && ownRegain > oppRegain {
		confidence := "medium"
		if gap >= 25 {
			confidence = "high"
		}
		return &performanceFinding{
			ID: findingID("press-effectiveness", own.ClubID), Type: "strength",
			Title:     "Pressing is winning the ball back",
			MetricKey: "press_success_rate",
			Evidence: fmt.Sprintf("Press success rate %.0f%% vs opponent's %.0f%%, with %.0f ball regains vs opponent's %.0f.",
				*ownRate.Value, *oppRate.Value, ownRegain, oppRegain),
			EventIDs: ids, ClipIDs: clipIDsFor(ids, clipEventIDs),
			Confidence: confidence, SampleSize: sampleSize,
		}
	}
	if gap <= -15 && ownRegain < oppRegain {
		confidence := "medium"
		if gap <= -25 {
			confidence = "high"
		}
		return &performanceFinding{
			ID: findingID("press-ineffectiveness", own.ClubID), Type: "weakness",
			Title:     "Pressing isn't winning the ball back",
			MetricKey: "press_success_rate",
			Evidence: fmt.Sprintf("Press success rate %.0f%% vs opponent's %.0f%%, with only %.0f ball regains vs opponent's %.0f.",
				*ownRate.Value, *oppRate.Value, ownRegain, oppRegain),
			EventIDs: ids, ClipIDs: clipIDsFor(ids, clipEventIDs),
			Confidence: confidence, SampleSize: sampleSize,
		}
	}
	return nil
}

// ruleSetPieceThreat: strength-only. Primary = set-piece chances+goals
// exceed the opponent's, off a minimum volume of corners/free-kicks.
// Corroboration = the resulting threat-per-set-piece rate clears a minimum
// efficiency floor (raw volume leading isn't enough on its own).
func ruleSetPieceThreat(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownCorners, _ := metricByKey(own.Attacking, "corners")
	ownFreeKicks, _ := metricByKey(own.Attacking, "free_kicks")
	setPieceVolume := ownCorners.SampleSize + ownFreeKicks.SampleSize
	if setPieceVolume < 3 {
		return nil
	}

	ownChances, _ := metricByKey(own.Attacking, "set_piece_chances_created")
	ownGoals, _ := metricByKey(own.Attacking, "set_piece_goals")
	oppChances, _ := metricByKey(opponent.Attacking, "set_piece_chances_created")
	oppGoals, _ := metricByKey(opponent.Attacking, "set_piece_goals")

	ownThreat := valueOrZero(ownChances.Value) + valueOrZero(ownGoals.Value)
	oppThreat := valueOrZero(oppChances.Value) + valueOrZero(oppGoals.Value)
	if ownThreat <= oppThreat {
		return nil
	}
	ownRate := ownThreat / float64(setPieceVolume)
	if ownRate < 0.25 {
		return nil
	}

	contributing := append(append([]models.AnalysisEvent{}, filterByType(own.Events, "set_piece_chance_created")...), filterByType(own.Events, "set_piece_goal")...)
	ids := eventIDs(contributing)
	confidence := "medium"
	if ownThreat-oppThreat >= 2 && ownRate >= 0.4 {
		confidence = "high"
	}
	return &performanceFinding{
		ID: findingID("set-piece-threat", own.ClubID), Type: "strength",
		Title:     "Set pieces are a genuine attacking weapon",
		MetricKey: "set_piece_chances_created",
		Evidence: fmt.Sprintf("%.0f set-piece chances/goals from %d corners/free-kicks (vs opponent's %.0f).",
			ownThreat, setPieceVolume, oppThreat),
		EventIDs: ids, ClipIDs: clipIDsFor(ids, clipEventIDs),
		Confidence: confidence, SampleSize: setPieceVolume,
	}
}

// ruleDefensiveErrorRisk: weakness-only. Primary = defensive_errors_general
// exceeds the opponent's (min sample 2). Corroboration = confidence is
// capped at "medium" unless at least one error is zone-tagged in the
// defensive third.
func ruleDefensiveErrorRisk(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownErrors, _ := metricByKey(own.Defensive, "defensive_errors_general")
	oppErrors, _ := metricByKey(opponent.Defensive, "defensive_errors_general")
	if ownErrors.SampleSize < 2 {
		return nil
	}
	if valueOrZero(ownErrors.Value) <= valueOrZero(oppErrors.Value) {
		return nil
	}

	errorEvents := filterByType(own.Events, "defensive_error", "failed_clearance", "failed_press", "failed_cover", "miscommunication", "wrong_decision")
	defensiveThirdCount := 0
	for _, e := range errorEvents {
		if zoneIndicatesDefensiveThird(e.PitchZone) {
			defensiveThirdCount++
		}
	}
	confidence := "medium"
	if defensiveThirdCount >= 1 {
		confidence = "high"
	}
	ids := eventIDs(errorEvents)
	return &performanceFinding{
		ID: findingID("defensive-error-risk", own.ClubID), Type: "weakness",
		Title:     "Defensive errors are creating risk",
		MetricKey: "defensive_errors_general",
		Evidence: fmt.Sprintf("%.0f defensive errors (vs opponent's %.0f), %d tagged in the defensive third.",
			valueOrZero(ownErrors.Value), valueOrZero(oppErrors.Value), defensiveThirdCount),
		EventIDs: ids, ClipIDs: clipIDsFor(ids, clipEventIDs),
		Confidence: confidence, SampleSize: ownErrors.SampleSize,
	}
}

// ruleDisciplineRisk: weakness-only. Primary = fouls_committed exceeds the
// opponent's (min sample 4). Corroboration = at least one card was actually
// shown - fouls alone never qualify.
func ruleDisciplineRisk(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownFouls, _ := metricByKey(own.Defensive, "fouls_committed")
	oppFouls, _ := metricByKey(opponent.Defensive, "fouls_committed")
	if ownFouls.SampleSize < 4 {
		return nil
	}
	if valueOrZero(ownFouls.Value) <= valueOrZero(oppFouls.Value) {
		return nil
	}

	cardEvents := filterByType(own.Events, "yellow_card", "second_yellow", "red_card")
	if len(cardEvents) < 1 {
		return nil
	}
	confidence := "medium"
	if len(cardEvents) >= 2 {
		confidence = "high"
	}

	foulEvents := filterByType(own.Events, "foul_committed")
	contributing := append(append([]models.AnalysisEvent{}, foulEvents...), cardEvents...)
	ids := eventIDs(contributing)
	return &performanceFinding{
		ID: findingID("discipline-risk", own.ClubID), Type: "weakness",
		Title:     "Discipline is costing cards",
		MetricKey: "fouls_committed",
		Evidence: fmt.Sprintf("%.0f fouls committed (vs opponent's %.0f), resulting in %d card(s).",
			valueOrZero(ownFouls.Value), valueOrZero(oppFouls.Value), len(cardEvents)),
		EventIDs: ids, ClipIDs: clipIDsFor(ids, clipEventIDs),
		Confidence: confidence, SampleSize: ownFouls.SampleSize,
	}
}

// ruleDuelDominance: strength-only. Primary = combined tackle/aerial/ground
// duel volume exceeds the opponent's by >=1.2x (min combined sample 8).
// Corroboration = ball-regain volume (recoveries + possession_won) also
// higher - winning duels must translate into actually regaining the ball.
func ruleDuelDominance(own, opponent teamMetricsBundle, clipEventIDs map[uint]bool) *performanceFinding {
	ownTackles, _ := metricByKey(own.Defensive, "tackles")
	ownAerial, _ := metricByKey(own.Defensive, "aerial_duels")
	ownGround, _ := metricByKey(own.Defensive, "ground_duels")
	ownDuels := ownTackles.SampleSize + ownAerial.SampleSize + ownGround.SampleSize
	if ownDuels < 8 {
		return nil
	}

	oppTackles, _ := metricByKey(opponent.Defensive, "tackles")
	oppAerial, _ := metricByKey(opponent.Defensive, "aerial_duels")
	oppGround, _ := metricByKey(opponent.Defensive, "ground_duels")
	oppDuels := oppTackles.SampleSize + oppAerial.SampleSize + oppGround.SampleSize
	if float64(ownDuels) <= float64(oppDuels)*1.2 {
		return nil
	}

	ownRecoveries, _ := metricByKey(own.Defensive, "recoveries")
	ownPossWon, _ := metricByKey(own.Defensive, "possession_won")
	oppRecoveries, _ := metricByKey(opponent.Defensive, "recoveries")
	oppPossWon, _ := metricByKey(opponent.Defensive, "possession_won")
	ownRegain := valueOrZero(ownRecoveries.Value) + valueOrZero(ownPossWon.Value)
	oppRegain := valueOrZero(oppRecoveries.Value) + valueOrZero(oppPossWon.Value)
	if ownRegain <= oppRegain {
		return nil
	}

	contributing := append(append(append([]models.AnalysisEvent{}, filterByType(own.Events, "tackle")...), filterByType(own.Events, "aerial_duel")...), filterByType(own.Events, "ground_duel")...)
	ids := eventIDs(contributing)
	confidence := "medium"
	if float64(ownDuels) >= float64(oppDuels)*1.5 {
		confidence = "high"
	}
	return &performanceFinding{
		ID: findingID("duel-dominance", own.ClubID), Type: "strength",
		Title:     "Winning the physical/duel battle",
		MetricKey: "tackles",
		Evidence: fmt.Sprintf("%d duels won (tackles/aerial/ground) vs opponent's %d, translating into %.0f ball regains vs %.0f.",
			ownDuels, oppDuels, ownRegain, oppRegain),
		EventIDs: ids, ClipIDs: clipIDsFor(ids, clipEventIDs),
		Confidence: confidence, SampleSize: ownDuels,
	}
}
