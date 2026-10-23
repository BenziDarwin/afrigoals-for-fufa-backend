# Scout accounts, search and AI trial

`scout` is a self-registerable role. It has no club/league management or analyst-write permissions. Its profile evidence is limited to approved or distributed match reports, while club managers can see the working match evidence already available through their club-scoped performance reports.

Authenticated scout/admin endpoints:

- `GET /api/v1/scout/players`: name/club/nationality query, position group, nationality, min/max age and page. Returns 24 real player identities plus approved/distributed tagged metrics, sample-aware scores and indicative values. Missing birth dates are not guessed. No monetary amount appears without the previously configured USD baseline.
- `GET /api/v1/scout/trial`: current account's remaining questions and saved answers.
- `POST /api/v1/scout/ai-assistant`: `{question, request_id, filters}`. Returns hints and verified recommended player IDs. A clear position in the question narrows the pool when no position filter was supplied. The assistant examines up to 24 filtered players prioritized by visible tagged-event count, not a global ranking; it can return no suitable candidates.

The trial allows three successful AI answers per account. A database row lock serializes quota reservations across requests/devices. Pending requests reserve a slot, failed requests release it, and stale reservations expire after ten minutes. Replaying a completed request ID returns the stored answer without another charge; reusing that ID for other criteria is rejected. Search and profile viewing remain available after the AI allowance is exhausted. There is no browser-storage quota or delete-history reset endpoint.

Uses the existing Anthropic SDK integration. `ANTHROPIC_API_KEY` is server-only; `SCOUT_AI_MODEL` optionally overrides the existing assistant model (`claude-opus-5`). Output IDs must belong to the supplied real-player pool. Draft events, guessed pace/footedness/contracts, and invented transfer values are excluded. Actual provider inference was not run in validation.

Migration `000016_scout_ai_insights` persists reservations and answers. It requires the existing migrations through 15. The local configured database has been brought from version 14 to 16. Transactional tests use a temporary schema, exercise migration up/down, quota exhaustion, retries, failed/expired reservations and separate-account allowances, then roll back all fixtures.

The database test also revealed that JSONB valuer methods returned byte arrays, which the configured PostgreSQL simple protocol rejected when saving arrays. They now return JSON strings, preserving JSONB storage and allowing scout recommendations and analyst report arrays to save correctly.

## CV data fixes

Club responses return nested GORM players with `ID`, while direct player responses use `id`. The frontend normalizes these records before opening a CV, so the evidence hook receives the real ID. All recognized roles can navigate to `/players/{id}`; the backend still controls evidence visibility. The club profile dialog links to the full CV. Scout discovery links to `/dashboard/scout/players/{id}` and preserves the existing CV panel.

Verification includes real database quota tests, player-filter and grounded-ID tests, frontend normalization/route tests, profile/refresh tests, TypeScript checks, and a local API build. The local API was restarted to load the fixes. Real AI responses and browser playback remain unverified.

Live verification through both the Go API and Next.js proxy returned HTTP 200 for player CV evidence, scout search and trial status. A team CV returned nine primary events and three linked clips. There are currently zero distributed reports, so scout statistics correctly remain empty until release (or a deliberate visibility-policy change).

Scout visibility policy: approval is sufficient for CV evidence and AI candidate metrics; distribution is not required. Draft, submitted, changes-requested and missing review states remain hidden.


## Delivering CVs to a scout

Open a player CV as an administrator, analyst, or that player's club manager.
Select **Send CV to scout**, enter the scout account's registered email, and
select **Send to dashboard**. The scout sees it under **Received player CVs**.
The inbox refreshes every 30 seconds and when the window regains focus.
Repeated sends by the same sender update one delivery rather than duplicate it.
Delivery links to the current profile and explicitly shares that player's pending evidence with the recipient, bounded by the sender's current access. Other scouts still see approved or distributed evidence only.

Migration 000017 adds the delivery table. Restart the API after updating.
API: POST /api/v1/scout-cv-deliveries ({player_id,email});
GET /api/v1/scout/received-cvs (current account only).


## Scout requests to teams

AI discovery queries database player identities and recorded analyst evidence,
including pending analysis, to recommend candidate IDs. It does not grant the
scout access to pending full profiles or clips. The candidate context remains
limited to 24 players prioritized by event count under the applied filters.

Scouts use **Request CV from team** beside AI recommendations or search cards.
The server routes requests using the player's current club ID. The owning
club manager sees **Scout CV requests** on their dashboard and can review the
player, share the CV, or decline. Sharing creates the existing recipient-only
delivery and marks the request shared atomically. Requesting alone grants no
additional access. Other clubs cannot view or handle the request. Scout request
status and received CVs refresh on focus and every 30 seconds.

Migration 000018 adds scout_cv_requests. API routes:
POST/GET /api/v1/scout-cv-requests and
POST /api/v1/scout-cv-requests/:id/respond ({action: share|decline}).


## Saved AI results and team request inbox

Completed AI answers and recommended player IDs are saved in scout_ai_insights,
scoped to the scout account. On refresh the dashboard restores the latest saved
answer and question from /api/v1/scout/trial. Previous answers have an Open saved
result action. Reloading a saved answer makes no model call and consumes no trial
question.

Team managers can open CV Requests from their side menu at
/dashboard/club-manager/cv-requests to review, grant CV access, or decline.
Granting uses the existing transactional request response endpoint and delivers
the profile to that requesting scout's account.
