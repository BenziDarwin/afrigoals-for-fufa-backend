CREATE TABLE scout_cv_requests (
 id BIGSERIAL PRIMARY KEY,
 scout_id BIGINT NOT NULL REFERENCES users(id),
 player_id BIGINT NOT NULL REFERENCES players(id),
 club_id BIGINT NOT NULL REFERENCES clubs(id),
 status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','shared','declined')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(scout_id,player_id,club_id)
);
CREATE INDEX idx_club_cv_requests ON scout_cv_requests(club_id,status);
