-- users.full_name backs the personalised greeting in the dashboard header.
-- Existing rows keep NULL and the UI falls back to the email local part.
ALTER TABLE public.users ADD COLUMN IF NOT EXISTS full_name character varying(150);

-- One shirt number per club.
--
-- The index is partial because players are soft deleted: without the WHERE
-- clause a deleted player would reserve their number permanently.
--
-- NOTE: this will fail on a database that already contains duplicates. That is
-- deliberate - the duplicates have to be resolved rather than silently kept.
-- To find them before migrating:
--
--   SELECT club_id, jersey_number, count(*)
--   FROM players
--   WHERE deleted_at IS NULL
--   GROUP BY club_id, jersey_number
--   HAVING count(*) > 1;
CREATE UNIQUE INDEX IF NOT EXISTS idx_players_club_jersey
    ON public.players USING btree (club_id, jersey_number)
    WHERE (deleted_at IS NULL);