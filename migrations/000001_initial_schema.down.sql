-- Reverse of 000001_initial_schema.up.sql.
--
-- CASCADE drops the dependent foreign keys, sequences and indexes with each
-- table, so the tables are listed leaf-first purely for readability.

-- Video & analysis
DROP TABLE IF EXISTS video_analysis_jobs CASCADE;
DROP TABLE IF EXISTS analysis_event_stats CASCADE;
DROP TABLE IF EXISTS analysis_events CASCADE;
DROP TABLE IF EXISTS clips CASCADE;
DROP TABLE IF EXISTS videos CASCADE;
DROP TABLE IF EXISTS match_videos CASCADE;
DROP TABLE IF EXISTS upload_sessions CASCADE;

-- Content
DROP TABLE IF EXISTS article_tags CASCADE;
DROP TABLE IF EXISTS articles CASCADE;

-- Football domain
DROP TABLE IF EXISTS match_events CASCADE;
DROP TABLE IF EXISTS unavailable_players CASCADE;
DROP TABLE IF EXISTS substitutes CASCADE;
DROP TABLE IF EXISTS lineup_players CASCADE;
DROP TABLE IF EXISTS formations CASCADE;
DROP TABLE IF EXISTS match_analysts CASCADE;
DROP TABLE IF EXISTS matches CASCADE;
DROP TABLE IF EXISTS player_stats CASCADE;
DROP TABLE IF EXISTS players CASCADE;
DROP TABLE IF EXISTS club_leagues CASCADE;
DROP TABLE IF EXISTS clubs CASCADE;

-- Base tables
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS leagues CASCADE;
