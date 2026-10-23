# Automatic player evidence and indicative value

`GET /api/v1/players/{id}/profile-analysis` is authenticated and uncached. It projects current analyst events, reports, physical observations and playable clips onto a stable player ID. No copying/backfill job or profile-stat write is needed: later edits, deletions and player reassignments appear on the next read. The frontend refreshes every 30 seconds and on focus.

Events use `player_id` and `secondary_player_id`, never player names or raw tracker numbers. Only primary actions contribute to performance totals; secondary involvement can supply clips. Repeated event IDs and report revisions do not inflate totals. Each player/match uses its latest saved report. Clips include completed database records, event URLs, and the existing R2-only clip-cutting workflow. R2 listing failures produce a warning and retain available linked clips. Unlinked clips are not guessed from their titles.

Admins and analysts can preview working evidence. Club managers see working evidence for matches they can already access. Scouts see approved or distributed reports across the player catalogue; league viewers retain distributed, ownership-scoped access. There is no shared response cache. The endpoint does not expose unassigned AI track statistics as player facts.

## Performance index

`analyst-outcomes-v1` is a transparent heuristic index (0–100), not an empirically trained player rating or peer percentile. It reuses existing sample-adjusted outcome metrics and reports their sample sizes, notes, weights and measured/estimated status.

| Position | Outcome weights |
| --- | --- |
| GK | Distribution .40, claims .30, recorded save outcomes .30 |
| DEF | Tackles .35, aerial duels .25, ground duels .20, passing .20 |
| MID | Passing .45, dribbling .25, ground duels .20, crosses .10 |
| FWD | Shot accuracy .40, dribbling .30, crosses .15, passing .15 |

Missing categories are omitted and remaining weights renormalised. Unknown positions or absent outcome data have no score. Scores remain provisional until at least three tagged matches, twenty classified outcome observations, two available categories, and 50% model-weight coverage. Tagging frequency and selective highlights can bias the index. Minutes and per-90 metrics are unavailable from event tags and are not invented. Physical figures are observation averages, not season totals.

## USD calibration

Set `PLAYER_VALUE_BASELINE_USD` to the chosen league reference value at index 50. Optional `PLAYER_VALUE_BASELINE_USD_GK`, `_DEF`, `_MID`, `_FWD` override it by position. Zero/missing/invalid values leave the profile at **Awaiting calibration**; this task intentionally does not choose a dollar baseline for the user.

With sufficient evidence: `estimate = round(baseline * (0.5 + index / 100))`. The displayed low/high scenario range uses ±`0.25 + 0.25*(1-coverage) + 0.25*(1-min(matches/10,1))`. It is a chosen evidence scenario range, not a statistical confidence interval. Sparse evidence produces no dollar estimate even after calibration. Model versions and calculation notes are returned in the API and displayed in profile details.

This first version does not model contract duration, age, league economics, demand or actual comparable transfer fees. It is therefore labelled **indicative value**, not a predicted fee. Established valuation methodologies use such additional context; see [Football Benchmark's methodology](https://footballbenchmark.com/player-valuation-methodology). Validate and recalibrate against relevant league transactions before treating the result as a market valuation.

## Validation and rollout

Six backend unit tests cover player isolation, report deduplication, clip safety/completion, secondary involvement, score coverage, USD calibration, role/distribution visibility, and endpoint authentication/ID validation. The opt-in read-only PostgreSQL test also passed against existing data: 18 linked events across one match; no playable clips for that fixture.

Run `go test ./services -run TestPlayerProfile -count=1`; set `PLAYER_PROFILE_INTEGRATION=1` to include the read-only database check. No schema migration is introduced. Restart/deploy the Go API and rebuild the Next.js frontend together. Modal needs no changes for this feature.

Scout visibility policy: approval is sufficient for CV evidence and AI candidate metrics; distribution is not required. Draft, submitted, changes-requested and missing review states remain hidden.
