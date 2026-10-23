# Persist analyst merge decisions and proxy review artifacts

Save accepted/rejected suggestion pairs in the existing versioned review document. Accepts become ordinary canonical merges and pass the existing cycle, role and player-assignment checks. Rejected pairs survive saves from older clients and disappear from review suggestions. AI result JSON remains unchanged.

Under `/api/v1/analyst-matches/{match_id}/video-jobs/{job_id}`, add authenticated routes for `GET /tracks/{track_id}/thumbnail`, `POST /reviewed-stats`, `GET/POST/PUT /jersey`, and `GET /jersey/crop`. Reviewed stats always use the saved database review and its version, rather than caller-supplied identities. Upstream URLs derive from the existing model artifact configuration.

Validation: `go test ./services -run 'TestTrackingReview|TestSuggestion' -count=1` passes. Tests cover normal merge rules, accepted chains, rejection persistence, unknown suggestions, conflicts and cycles. PostgreSQL integration and a live Modal service were not exercised.

Requires the companion FastAPI artifact endpoints and Next.js review tools. No migration is needed: decisions live in the existing review JSON document and revision history.
