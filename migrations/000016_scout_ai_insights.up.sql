CREATE TABLE IF NOT EXISTS scout_ai_insights (
 id bigserial PRIMARY KEY,
 user_id bigint NOT NULL REFERENCES users(id),
 request_key varchar(64) NOT NULL,
 criteria_hash varchar(64) NOT NULL,
 question text NOT NULL,
 answer text NOT NULL DEFAULT '',
 player_ids jsonb NOT NULL DEFAULT '[]',
 status varchar(20) NOT NULL CHECK (status IN ('pending','completed','failed')),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT idx_scout_request UNIQUE (user_id,request_key)
);
CREATE INDEX IF NOT EXISTS idx_scout_usage ON scout_ai_insights(user_id,status,created_at);
