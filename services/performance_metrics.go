package services

import (
	"fmt"
	"strings"

	"afrigoals.com/models"
)

// PerformanceStatus tags every performanceMetric so the report never
// presents weak or absent data as if it were solid. See the mapping notes
// on each compute*Metrics function for which EventType values back which
// status.
type PerformanceStatus string

const (
	// StatusMeasured is a direct count, or a ratio of two direct counts,
	// from first-class EventType.Value tags (e.g. pass_completed/pass_failed).
	StatusMeasured PerformanceStatus = "measured"
	// StatusEstimated is derived from the free-text Outcome field (no
	// controlled vocabulary exists) or is a same-match ratio proxy rather
	// than a true linked-event outcome.
	StatusEstimated PerformanceStatus = "estimated"
	// StatusUnavailable means the schema fundamentally cannot support this
	// metric (no x/y coordinates, no linked-event chain, no minutes-played
	// field anywhere). Value is always nil - never a fabricated zero.
	StatusUnavailable PerformanceStatus = "unavailable"
)

type performanceMetric struct {
	Key        string            `json:"key"`
	Label      string            `json:"label"`
	Category   string            `json:"category"`
	Status     PerformanceStatus `json:"status"`
	Value      *float64          `json:"value"`
	Unit       string            `json:"unit,omitempty"`
	SampleSize int               `json:"sample_size"`
	Notes      string            `json:"notes,omitempty"`
	EventIDs   []uint            `json:"event_ids,omitempty"`
}

type metricSection struct {
	Metrics []performanceMetric `json:"metrics"`
}

func metricByKey(section metricSection, key string) (performanceMetric, bool) {
	for _, m := range section.Metrics {
		if m.Key == key {
			return m, true
		}
	}
	return performanceMetric{}, false
}

// resolveEventClub returns the club an event belongs to, preferring the
// event's own TeamID and falling back to the tagged player's club.
//
// This distinction matters: migrations/000005_event_types_and_event_metadata
// seeds a whole tranche of transition/tactical EventType values (high_press,
// mid_block, low_block, counter_attack, fast_break, attacking_transition,
// defensive_transition, time_to_shoot_after_recovery, and the tactical-
// context tags) with requires_player=false - they are tagged team-level via
// AnalysisEvent.TeamID, with no PlayerID at all. Attributing purely by
// PlayerID (as computeTeamPerformanceSummaries in
// player_analysis_reports.go does for its own, unrelated purpose) would
// silently drop every one of those events from team aggregation.
func resolveEventClub(event models.AnalysisEvent, playerClub map[uint]uint) (uint, bool) {
	if event.TeamID != nil && *event.TeamID != 0 {
		return *event.TeamID, true
	}
	if event.PlayerID != nil {
		if clubID, ok := playerClub[*event.PlayerID]; ok {
			return clubID, true
		}
	}
	return 0, false
}

// bucketEventsByClub splits a match's events into home/away using
// resolveEventClub. Events that resolve to neither club (bad/legacy data)
// land in unattributed rather than being silently dropped, so callers can
// surface that as a data-quality signal instead of losing it quietly.
func bucketEventsByClub(events []models.AnalysisEvent, homeClubID, awayClubID uint, playerClub map[uint]uint) (home, away, unattributed []models.AnalysisEvent) {
	for _, e := range events {
		clubID, ok := resolveEventClub(e, playerClub)
		switch {
		case !ok:
			unattributed = append(unattributed, e)
		case clubID == homeClubID:
			home = append(home, e)
		case clubID == awayClubID:
			away = append(away, e)
		default:
			unattributed = append(unattributed, e)
		}
	}
	return home, away, unattributed
}

func filterByType(events []models.AnalysisEvent, values ...string) []models.AnalysisEvent {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	var out []models.AnalysisEvent
	for _, e := range events {
		if set[e.Type] {
			out = append(out, e)
		}
	}
	return out
}

func eventIDs(events []models.AnalysisEvent) []uint {
	if len(events) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	return ids
}

// successOutcomes/failureOutcomes are the fixed, small vocabulary
// classifyOutcome recognizes in the free-text Outcome field. There is no
// schema-enforced controlled vocabulary for Outcome, so anything outside
// this list is left unclassified rather than guessed.
var successOutcomes = map[string]bool{
	"successful": true, "success": true, "completed": true, "complete": true, "won": true,
}
var failureOutcomes = map[string]bool{
	"unsuccessful": true, "failed": true, "failure": true, "lost": true, "incomplete": true, "missed": true,
}

// classifyOutcome reads the free-text Outcome field against a fixed
// vocabulary (case-insensitive, trimmed). classified is false for nil,
// empty, or any string outside the vocabulary - callers must never treat
// an unclassified outcome as a failure.
func classifyOutcome(outcome *string) (success bool, classified bool) {
	if outcome == nil {
		return false, false
	}
	v := strings.ToLower(strings.TrimSpace(*outcome))
	if v == "" {
		return false, false
	}
	if successOutcomes[v] {
		return true, true
	}
	if failureOutcomes[v] {
		return false, true
	}
	return false, false
}

func countMetric(key, label, category string, matching []models.AnalysisEvent) performanceMetric {
	v := float64(len(matching))
	return performanceMetric{
		Key: key, Label: label, Category: category,
		Status: StatusMeasured, Value: &v, Unit: "count",
		SampleSize: len(matching), EventIDs: eventIDs(matching),
	}
}

const percentageConfidenceSample = 2.0

func confidenceScaledPercent(numeratorCount, denominatorCount int) float64 {
	if denominatorCount == 0 {
		return 0
	}
	raw := 100 * float64(numeratorCount) / float64(denominatorCount)
	confidence := float64(denominatorCount) / (float64(denominatorCount) + percentageConfidenceSample)
	return raw * confidence
}

// ratioMetric expresses len(numerator)/len(denominator) as a percentage.
// numerator is expected to be a subset of denominator. The displayed value is
// confidence-scaled by sample size, so a tiny perfect sample does not outrank
// a larger, more representative team performance. Value is nil (never zero)
// when the denominator is empty.
func ratioMetric(key, label, category string, numerator, denominator []models.AnalysisEvent, notes string) performanceMetric {
	if len(denominator) == 0 {
		return performanceMetric{
			Key: key, Label: label, Category: category,
			Status: StatusMeasured, Value: nil, Unit: "percent",
			SampleSize: 0, Notes: notes,
		}
	}
	v := confidenceScaledPercent(len(numerator), len(denominator))
	scaledNotes := strings.TrimSpace(notes)
	scaleNote := fmt.Sprintf("Performance-weighted by sample size; raw rate was %.0f%% from %d/%d events.", 100*float64(len(numerator))/float64(len(denominator)), len(numerator), len(denominator))
	if scaledNotes == "" {
		scaledNotes = scaleNote
	} else {
		scaledNotes += " " + scaleNote
	}
	return performanceMetric{
		Key: key, Label: label, Category: category,
		Status: StatusMeasured, Value: &v, Unit: "percent",
		SampleSize: len(denominator), Notes: scaledNotes,
		EventIDs: eventIDs(denominator),
	}
}

// estimatedOutcomeRateMetric reads the free-text Outcome field of `attempts`
// through classifyOutcome. Only classifiable attempts count toward the
// rate's denominator - unclassifiable ones are excluded, never guessed.
func estimatedOutcomeRateMetric(key, label, category string, attempts []models.AnalysisEvent) performanceMetric {
	var classifiedIDs []uint
	successCount, classifiedCount := 0, 0
	for _, e := range attempts {
		success, classified := classifyOutcome(e.Outcome)
		if !classified {
			continue
		}
		classifiedCount++
		classifiedIDs = append(classifiedIDs, e.ID)
		if success {
			successCount++
		}
	}
	notes := fmt.Sprintf("Estimated from the free-text outcome field; %d of %d tagged attempts had a classifiable outcome.", classifiedCount, len(attempts))
	if classifiedCount == 0 {
		return performanceMetric{
			Key: key, Label: label, Category: category,
			Status: StatusEstimated, Value: nil, Unit: "percent",
			SampleSize: 0, Notes: notes, EventIDs: eventIDs(attempts),
		}
	}
	v := confidenceScaledPercent(successCount, classifiedCount)
	return performanceMetric{
		Key: key, Label: label, Category: category,
		Status: StatusEstimated, Value: &v, Unit: "percent",
		SampleSize: classifiedCount, Notes: notes, EventIDs: classifiedIDs,
	}
}

func unavailableMetric(key, label, category, reason string) performanceMetric {
	return performanceMetric{
		Key: key, Label: label, Category: category,
		Status: StatusUnavailable, Value: nil, Notes: reason,
	}
}

// perNinety returns count normalized to a 90-minute rate, or nil if
// minutesPlayed is nil or non-positive. Phase 1 has no minutes-played data
// source anywhere in the schema (Substitute, LineupPlayer, and
// AnalysisEventStats all lack a time-on-pitch field), so every current
// caller always passes nil and must render "unavailable", never 0. This is
// implemented now as a pure, tested utility so a future minutes-played data
// source can enable it without touching call sites.
func perNinety(count int, minutesPlayed *float64) *float64 {
	if minutesPlayed == nil || *minutesPlayed <= 0 {
		return nil
	}
	v := float64(count) * 90 / *minutesPlayed
	return &v
}

// computeAttackingMetrics maps event_types categories 'attack', 'passing',
// plus attacking-relevant 'possession'/'movement'/'set_piece' values, onto
// the spec's attacking-performance section. See migrations/000005 for the
// full seeded taxonomy this reads.
func computeAttackingMetrics(events []models.AnalysisEvent) metricSection {
	shots := filterByType(events, "shot", "shot_on_target", "goal")
	shotsOnTarget := filterByType(events, "shot_on_target", "goal")
	goals := filterByType(events, "goal")
	disallowedGoals := filterByType(events, "goal_disallowed")
	bigChances := filterByType(events, "big_chance", "goal", "shot_on_target", "goal_disallowed")
	crosses := filterByType(events, "cross")
	dribbles := filterByType(events, "dribble")
	passCompleted := filterByType(events, "pass_completed")
	passFailed := filterByType(events, "pass_failed")
	passAttempts := append(append([]models.AnalysisEvent{}, passCompleted...), passFailed...)

	metrics := []performanceMetric{
		countMetric("goals", "Goals", "attack", goals),
		countMetric("disallowed_goals", "Disallowed Goals", "attack", disallowedGoals),
		countMetric("shots_total", "Total Shots", "attack", shots),
		countMetric("shots_on_target", "Shots on Target", "attack", shotsOnTarget),
		ratioMetric("shot_accuracy_pct", "Shot Accuracy %", "attack", shotsOnTarget, shots,
			"Assumes analysts tag exactly one of shot/shot_on_target/goal per attempt - a tagging convention, not a schema constraint."),
		countMetric("big_chances", "Big Chances", "attack", bigChances),
		estimatedRatio("big_chance_conversion_pct", "Big Chance Conversion %", "attack", goals, bigChances,
			"Ratio proxy only - no event links a specific goal to a specific big chance."),
		countMetric("assists", "Assists", "attack", filterByType(events, "assist")),
		countMetric("key_passes", "Key Passes", "attack", filterByType(events, "key_pass")),
		countMetric("through_balls", "Through Balls", "attack", filterByType(events, "through_ball")),
		countMetric("crosses", "Crosses", "attack", crosses),
		estimatedOutcomeRateMetric("cross_success_pct", "Cross Success %", "attack", crosses),
		countMetric("cutbacks", "Cutbacks", "attack", filterByType(events, "cutback")),
		countMetric("dribbles", "Dribbles", "attack", dribbles),
		estimatedOutcomeRateMetric("dribble_success_pct", "Dribble Success %", "attack", dribbles),
		countMetric("one_v_one_attacks", "1v1 Attacking Actions", "attack", filterByType(events, "one_v_one_attack")),
		ratioMetric("pass_completion_pct", "Pass Completion %", "passing", passCompleted, passAttempts,
			"Directly measured from the pass_completed/pass_failed split - the most reliable ratio in the taxonomy."),
		countMetric("progressive_passes", "Progressive Passes", "passing", filterByType(events, "progressive_pass")),
		countMetric("long_balls", "Long Balls", "passing", filterByType(events, "long_ball")),
		countMetric("switches_of_play", "Switches of Play", "passing", filterByType(events, "switch_of_play")),
		countMetric("passes_into_final_third", "Passes into Final Third", "passing", filterByType(events, "pass_into_final_third")),
		countMetric("passes_into_penalty_area", "Passes into Penalty Area", "passing", filterByType(events, "pass_into_penalty_area")),
		countMetric("one_touch_passes", "One-Touch Passes", "passing", filterByType(events, "one_touch_pass")),
		countMetric("combination_plays", "Combination Plays", "passing", filterByType(events, "combination_play")),
		countMetric("ball_carries", "Ball Carries", "possession", filterByType(events, "ball_carry")),
		countMetric("carries_into_final_third", "Carries into Final Third", "possession", filterByType(events, "carry_into_final_third")),
		countMetric("attacking_off_ball_runs", "Attacking/Overlap/Underlap Runs", "movement",
			filterByType(events, "attacking_run", "overlap_run", "underlap_run", "run_behind_defence", "support_run", "line_breaking_movement")),
		countMetric("offsides", "Offsides", "movement", filterByType(events, "offside")),
		countMetric("set_piece_chances_created", "Set-Piece Chances Created", "set_piece", filterByType(events, "set_piece_chance_created")),
		countMetric("set_piece_goals", "Set-Piece Goals", "set_piece", filterByType(events, "set_piece_goal")),
		countMetric("corners", "Corners", "set_piece", filterByType(events, "corner_kick")),
		countMetric("free_kicks", "Free Kicks", "set_piece", filterByType(events, "free_kick")),
		countMetric("penalties", "Penalties", "set_piece", filterByType(events, "penalty")),
		unavailableMetric("expected_goals_xg", "Expected Goals (xG)", "attack",
			"Requires shot location (x/y) and placement data, which the schema does not capture."),
		unavailableMetric("progressive_carry_distance", "Progressive Carry Distance", "possession",
			"Requires numeric pitch coordinates - only a free-text pitch_zone label exists."),
		unavailableMetric("shot_zone_map", "Shot Location Map", "attack",
			"Requires shot-specific x/y coordinates, which the schema does not capture."),
	}
	return metricSection{Metrics: metrics}
}

// computeDefensiveMetrics maps event_types categories 'defence',
// 'goalkeeper', 'discipline', plus defensively-relevant 'possession'/'error'
// values, onto the spec's defensive-performance section. "Conceded" metrics
// (shots/goals conceded, save %) need the opponent's attacking numbers and
// are injected afterward by injectConcededMetrics.
func computeDefensiveMetrics(events []models.AnalysisEvent) metricSection {
	tackles := filterByType(events, "tackle")
	aerialDuels := filterByType(events, "aerial_duel")
	groundDuels := filterByType(events, "ground_duel")
	saves := filterByType(events, "save", "one_vs_one_save")

	metrics := []performanceMetric{
		countMetric("tackles", "Tackles", "defence", tackles),
		estimatedOutcomeRateMetric("tackle_success_pct", "Tackle Success %", "defence", tackles),
		countMetric("interceptions", "Interceptions", "defence", filterByType(events, "interception")),
		countMetric("clearances", "Clearances", "defence", filterByType(events, "clearance")),
		countMetric("blocks", "Blocks", "defence", filterByType(events, "block")),
		countMetric("aerial_duels", "Aerial Duels", "defence", aerialDuels),
		estimatedOutcomeRateMetric("aerial_duel_success_pct", "Aerial Duel Success %", "defence", aerialDuels),
		countMetric("ground_duels", "Ground Duels", "defence", groundDuels),
		estimatedOutcomeRateMetric("ground_duel_success_pct", "Ground Duel Success %", "defence", groundDuels),
		countMetric("recoveries", "Recoveries", "defence", filterByType(events, "recovery")),
		countMetric("possession_won", "Possession Won", "possession", filterByType(events, "possession_won")),
		countMetric("marking_errors", "Marking Errors", "defence", filterByType(events, "marking_error")),
		countMetric("positional_errors", "Positional Errors", "defence", filterByType(events, "positional_error")),
		countMetric("last_man_defence", "Last-Man Defensive Actions", "defence", filterByType(events, "last_man_defence")),
		countMetric("goal_line_clearances", "Goal-Line Clearances", "defence", filterByType(events, "goal_line_clearance")),
		countMetric("defensive_errors_general", "Defensive Errors", "error",
			filterByType(events, "defensive_error", "failed_clearance", "failed_press", "failed_cover", "miscommunication", "wrong_decision")),
		countMetric("saves", "Saves", "goalkeeper", saves),
		countMetric("gk_claims_catches", "Claims/Catches/Punches", "goalkeeper", filterByType(events, "catch_cross", "punch", "claim_high_ball")),
		countMetric("gk_distribution", "GK Distribution", "goalkeeper", filterByType(events, "distribution_short", "distribution_long", "throw_distribution")),
		countMetric("sweeper_actions", "Sweeper Actions", "goalkeeper", filterByType(events, "sweeper_action")),
		countMetric("goalkeeper_errors", "Goalkeeper Errors", "goalkeeper", filterByType(events, "goalkeeper_error")),
		countMetric("fouls_committed", "Fouls Committed", "discipline", filterByType(events, "foul_committed")),
		countMetric("fouls_suffered", "Fouls Suffered", "discipline", filterByType(events, "foul_suffered")),
		countMetric("cards", "Cards", "discipline", filterByType(events, "yellow_card", "second_yellow", "red_card")),
		unavailableMetric("ppda", "Passes Per Defensive Action", "defence",
			"Requires the opposition's full pass log plus zone-boxed possession phases, which the schema does not capture."),
		unavailableMetric("defensive_line_height", "Avg. Defensive Line Height", "defence",
			"Requires x/y tracking data, which the schema does not capture."),
	}
	return metricSection{Metrics: metrics}
}

// injectConcededMetrics appends "conceded" defensive metrics that need the
// opponent's attacking numbers (shots conceded, shots on target conceded,
// goals conceded), plus a save percentage now that goals conceded is known.
func injectConcededMetrics(defensive metricSection, opponentAttacking metricSection) metricSection {
	shotsConceded, _ := metricByKey(opponentAttacking, "shots_total")
	shotsOnTargetConceded, _ := metricByKey(opponentAttacking, "shots_on_target")
	goalsConceded, _ := metricByKey(opponentAttacking, "goals")
	saves, _ := metricByKey(defensive, "saves")

	defensive.Metrics = append(defensive.Metrics,
		renamedMetric(shotsConceded, "shots_conceded", "Shots Conceded", "defence"),
		renamedMetric(shotsOnTargetConceded, "shots_on_target_conceded", "Shots on Target Conceded", "defence"),
		renamedMetric(goalsConceded, "goals_conceded", "Goals Conceded", "defence"),
		savePercentMetric(saves, goalsConceded),
	)
	return defensive
}

func renamedMetric(m performanceMetric, key, label, category string) performanceMetric {
	m.Key = key
	m.Label = label
	m.Category = category
	return m
}

func savePercentMetric(saves, goalsConceded performanceMetric) performanceMetric {
	saveCount := valueOrZero(saves.Value)
	concededCount := valueOrZero(goalsConceded.Value)
	total := saveCount + concededCount
	if total == 0 {
		return performanceMetric{
			Key: "save_pct", Label: "Save %", Category: "goalkeeper",
			Status: StatusEstimated, Value: nil, Unit: "percent", SampleSize: 0,
			Notes: "No shots faced were tagged for this match.",
		}
	}
	v := 100 * saveCount / total
	return performanceMetric{
		Key: "save_pct", Label: "Save %", Category: "goalkeeper",
		Status: StatusEstimated, Value: &v, Unit: "percent", SampleSize: int(total),
		Notes: "Match-level proxy only - no event links a specific shot faced to a specific save.",
	}
}

// computeTransitionMetrics maps the 'transition' category plus
// transition-relevant 'possession' turnover values and 'tactical' context
// tags onto the spec's transition-performance section.
func computeTransitionMetrics(events []models.AnalysisEvent) metricSection {
	pressingActions := filterByType(events, "pressing_action")
	pressSuccesses := filterByType(events, "press_success")

	metrics := []performanceMetric{
		countMetric("high_press_actions", "High Press", "transition", filterByType(events, "high_press")),
		countMetric("mid_block_tags", "Mid Block", "transition", filterByType(events, "mid_block")),
		countMetric("low_block_tags", "Low Block", "transition", filterByType(events, "low_block")),
		countMetric("pressing_actions", "Pressing Actions", "transition", pressingActions),
		countMetric("press_successes", "Press Success", "transition", pressSuccesses),
		pressSuccessRateMetric(pressSuccesses, pressingActions),
		countMetric("counter_presses", "Counter Press", "transition", filterByType(events, "counter_press")),
		countMetric("counter_attacks", "Counter Attacks", "transition", filterByType(events, "counter_attack")),
		countMetric("fast_breaks", "Fast Breaks", "transition", filterByType(events, "fast_break")),
		countMetric("attacking_transitions", "Attacking Transitions", "transition", filterByType(events, "attacking_transition")),
		countMetric("defensive_transitions", "Defensive Transitions", "transition", filterByType(events, "defensive_transition")),
		countMetric("recoveries_after_loss", "Recoveries After Loss", "transition", filterByType(events, "recovery_after_loss")),
		countMetric("time_to_shoot_after_recovery_tags", `"Time to Shoot After Recovery" Moments`, "transition", filterByType(events, "time_to_shoot_after_recovery")),
		countMetric("possession_lost", "Possession Lost", "possession", filterByType(events, "possession_lost")),
		countMetric("dangerous_turnovers", "Dangerous Turnovers", "possession", filterByType(events, "dangerous_turnover")),
		countMetric("bad_touches_miscontrols", "Bad Touches / Miscontrols", "possession", filterByType(events, "bad_touch", "miscontrol")),
		countMetric("pressure_received", "Pressure Received", "possession", filterByType(events, "pressure_received")),
		countMetric("pressure_escaped", "Pressure Escaped", "possession", filterByType(events, "pressure_escaped")),
		countMetric("tactical_context_tags", "Tactical Context Tags", "tactical", filterByType(events,
			"formation_change", "pressing_style_change", "defensive_shape", "attacking_pattern",
			"build_up_phase", "possession_phase", "transition_phase", "player_rotation",
			"numerical_advantage_created", "numerical_disadvantage_created")),
		unavailableMetric("avg_time_to_shoot_after_recovery_seconds", "Avg. Time to Shoot After Recovery", "transition",
			"No duration/linked-event field exists to compute elapsed seconds between a recovery and a shot."),
		unavailableMetric("counter_attack_conversion", "Counter-Attack Conversion", "transition",
			"Deriving this from independent event timestamps would risk false attribution when multiple counters/shots occur close together; no linked-event chain exists in the schema."),
		unavailableMetric("transition_speed_seconds", "Turnover to Shot Time", "transition",
			"No duration/linked-event field exists in the schema."),
	}
	return metricSection{Metrics: metrics}
}

func pressSuccessRateMetric(pressSuccesses, pressingActions []models.AnalysisEvent) performanceMetric {
	denominator := append(append([]models.AnalysisEvent{}, pressSuccesses...), pressingActions...)
	return ratioMetric("press_success_rate", "Press Success Rate", "transition", pressSuccesses, denominator,
		"The taxonomy has no press_failed counterpart to press_success, so this assumes analysts tag every pressing moment as either pressing_action or press_success, never both for the same moment.")
}

// estimatedRatio is ratioMetric with its status forced to "estimated" - for
// ratios that are numerically a count/count division but conceptually a
// same-match proxy rather than a true linked-event outcome (e.g. big-chance
// conversion, where no event links a specific goal to a specific chance).
func estimatedRatio(key, label, category string, numerator, denominator []models.AnalysisEvent, notes string) performanceMetric {
	m := ratioMetric(key, label, category, numerator, denominator, notes)
	m.Status = StatusEstimated
	return m
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// zoneFrequency tallies non-empty PitchZone tags, for team- or player-scoped
// heatmaps. Events with no zone tag are excluded from the tally entirely -
// never bucketed under a fabricated "unknown" zone.
func zoneFrequency(events []models.AnalysisEvent) map[string]int {
	freq := map[string]int{}
	for _, e := range events {
		if e.PitchZone == nil {
			continue
		}
		zone := strings.TrimSpace(*e.PitchZone)
		if zone == "" {
			continue
		}
		freq[zone]++
	}
	return freq
}
