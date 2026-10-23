CREATE TABLE scout_cv_deliveries (
 id BIGSERIAL PRIMARY KEY,
 scout_id BIGINT NOT NULL REFERENCES users(id),
 player_id BIGINT NOT NULL REFERENCES players(id),
 sender_id BIGINT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(scout_id, player_id, sender_id)
);
CREATE INDEX idx_scout_cv_inbox ON scout_cv_deliveries(scout_id, created_at DESC);
