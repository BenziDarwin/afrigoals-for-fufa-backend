-- Uganda v Botswana, 14 November 2026, Hoima City Stadium.
-- Starting XIs and benches follow the supplied match list, checked against
-- https://www.espn.com/soccer/lineups/_/gameId/401927188
-- Right/left roles follow the match record. Coordinates use the application's
-- existing formation grid. Uganda 3-4-3, Botswana 4-4-2.
-- Only the formation records for this exact fixture are changed.
BEGIN;
SET LOCAL lock_timeout='10s';
SET LOCAL statement_timeout='60s';
SELECT pg_advisory_xact_lock(hashtextextended('afrigoals-uganda-botswana-20261114-lineups',0));
SELECT id FROM public.clubs WHERE id IN(13,37) ORDER BY id FOR UPDATE;
CREATE TEMP TABLE matchday_squad (
 team_id bigint NOT NULL,player_id bigint NOT NULL,expected_name text NOT NULL,
 shirt integer NOT NULL,starter boolean NOT NULL,match_position text NOT NULL,
 x numeric,y numeric,PRIMARY KEY(team_id,player_id),UNIQUE(team_id,shirt)
) ON COMMIT DROP;
INSERT INTO matchday_squad VALUES
 (13, 200, 'David Lukwago', 1, false, 'Goalkeeper', NULL, NULL),
 (13, 412, 'Nathan Asiimwe', 2, true, 'Right Midfielder', 20, 50),
 (13, 413, 'Rogers Torach', 3, true, 'Center Defender', 50, 25),
 (13, 414, 'Kenneth Ssemakula', 4, true, 'Center Right Defender', 25, 25),
 (13, 415, 'Toby Sibbick', 5, true, 'Center Left Defender', 75, 25),
 (13, 416, 'Bobosi Byaruhanga', 6, true, 'Center Left Midfielder', 60, 50),
 (13, 417, 'Sadam Masereka', 7, false, 'Winger', NULL, NULL),
 (13, 418, 'Khalid Aucho', 8, false, 'Midfielder', NULL, NULL),
 (13, 419, 'Arnold Vula', 9, true, 'Forward', 50, 75),
 (13, 420, 'Travis Mutyaba', 10, false, 'Attacking Midfielder', NULL, NULL),
 (13, 421, 'Herbert Bockhorn', 11, false, 'Right Back', NULL, NULL),
 (13, 422, 'Kelvin Boateng', 13, true, 'Center Right Forward', 80, 75),
 (13, 423, 'Denis Omedi', 14, false, 'Forward', NULL, NULL),
 (13, 424, 'Baba Alhassan', 15, true, 'Center Right Midfielder', 40, 50),
 (13, 93, 'Hilary Mukundane', 16, false, 'Defender', NULL, NULL),
 (13, 425, 'James Bogere', 17, true, 'Center Left Forward', 20, 75),
 (13, 426, 'Denis Onyango', 18, true, 'Goalkeeper', 50, 5),
 (13, 427, 'Kingdom Osayi', 19, false, 'Goalkeeper', NULL, NULL),
 (13, 428, 'Toto Majub', 20, false, 'Center Back', NULL, NULL),
 (13, 260, 'Allan Okello', 21, false, 'Winger', NULL, NULL),
 (13, 429, 'Allan Oyirwoth', 22, false, 'Midfielder', NULL, NULL),
 (13, 430, 'Aziz Kayondo', 23, true, 'Left Midfielder', 80, 50),
 (13, 205, 'Frank Ssebufu', 24, false, 'Forward', NULL, NULL),
 (37, 389, 'Victor James', 1, false, 'Goalkeeper', NULL, NULL),
 (37, 390, 'Botsile Sakana', 2, false, 'Center Back', NULL, NULL),
 (37, 391, 'Larona Modisaemang', 3, false, 'Defender', NULL, NULL),
 (37, 392, 'Mosha Gaolaolwe', 4, true, 'Center Left Defender', 65, 25),
 (37, 393, 'Alford Velaphi', 5, true, 'Right Back', 15, 25),
 (37, 394, 'Gape Mohutsiwa', 6, true, 'Center Right Midfielder', 35, 50),
 (37, 395, 'Losika Ratshukudu', 7, true, 'Left Midfielder', 85, 50),
 (37, 396, 'Amogelang Reetsang', 8, false, 'Midfielder', NULL, NULL),
 (37, 397, 'Godfrey Tauyatswala', 9, false, 'Forward', NULL, NULL),
 (37, 398, 'Thero Setsile', 10, true, 'Right Midfielder', 15, 50),
 (37, 399, 'Tumisang Orebonye', 11, true, 'Center Right Forward', 35, 75),
 (37, 400, 'Thatayaone Kgamanyane', 12, false, 'Forward', NULL, NULL),
 (37, 401, 'Kutlwelo Mpolokang', 13, false, 'Midfielder', NULL, NULL),
 (37, 402, 'Godiraone Modingwane', 14, true, 'Center Left Midfielder', 65, 50),
 (37, 403, 'Omphile Ramoagi', 15, false, 'Midfielder', NULL, NULL),
 (37, 404, 'Kenneth Mmoko', 16, false, 'Goalkeeper', NULL, NULL),
 (37, 405, 'Mbatshi Elias', 17, false, 'Forward', NULL, NULL),
 (37, 406, 'Lebogang Ditsele', 18, false, 'Defensive Midfielder', NULL, NULL),
 (37, 407, 'Chicco Molefe', 19, true, 'Left Back', 85, 25),
 (37, 408, 'Tebogo Kopelang', 20, true, 'Center Right Defender', 35, 25),
 (37, 409, 'Thabo Maponda', 21, false, 'Forward', NULL, NULL),
 (37, 410, 'Gilbert Baruti', 22, true, 'Center Left Forward', 65, 75),
 (37, 411, 'Goitseone Phoko', 23, true, 'Goalkeeper', 50, 5);

DO $lineups$
DECLARE
 v_match_id bigint;
 v_formation_id bigint;
 v_count integer;
 v_ids bigint[];
 team record;
BEGIN
 SELECT array_agg(id) INTO v_ids FROM public.matches WHERE deleted_at IS NULL
   AND home_club_id=13 AND away_club_id=37 AND (date AT TIME ZONE 'UTC')::date=DATE '2026-11-14';
 IF cardinality(v_ids) IS DISTINCT FROM 1 THEN
  RAISE EXCEPTION 'Expected exactly one Uganda v Botswana fixture on 14 November 2026';
 END IF;
 v_match_id:=v_ids[1];
 PERFORM id FROM public.matches WHERE id=v_match_id FOR UPDATE;
 IF (SELECT count(*) FROM matchday_squad)<>46 OR EXISTS (
  SELECT 1 FROM matchday_squad GROUP BY team_id
  HAVING count(*)<>23 OR count(*) FILTER(WHERE starter)<>11 OR count(*) FILTER(WHERE NOT starter)<>12
 ) THEN RAISE EXCEPTION 'Each squad must contain 11 starters and 12 substitutes'; END IF;
 SELECT count(*) INTO v_count FROM matchday_squad s JOIN public.players p ON p.id=s.player_id
 JOIN public.national_callups n ON n.player_id=p.id AND n.team_id=s.team_id AND n.withdrawn_at IS NULL
 WHERE p.deleted_at IS NULL AND p.name=s.expected_name AND p.national_team_id=s.team_id
   AND p.national_jersey_number=s.shirt;
 IF v_count<>46 THEN RAISE EXCEPTION 'A player identity, national shirt or call-up has changed'; END IF;

 FOR team IN SELECT DISTINCT team_id FROM matchday_squad ORDER BY team_id LOOP
  SELECT array_agg(id) INTO v_ids FROM public.formations
  WHERE match_id=v_match_id AND club_id=team.team_id AND deleted_at IS NULL;
  IF cardinality(v_ids)>1 THEN RAISE EXCEPTION 'Duplicate active formations need review for team %',team.team_id; END IF;
  IF cardinality(v_ids)=1 THEN
   v_formation_id:=v_ids[1];
   UPDATE public.formations SET formation=CASE team.team_id WHEN 13 THEN '3-4-3' ELSE '4-4-2' END,updated_at=now()
   WHERE id=v_formation_id AND formation IS DISTINCT FROM CASE team.team_id WHEN 13 THEN '3-4-3' ELSE '4-4-2' END;
  ELSE
   INSERT INTO public.formations(uuid,match_id,club_id,formation,created_at,updated_at)
   VALUES(gen_random_uuid(),v_match_id,team.team_id,CASE team.team_id WHEN 13 THEN '3-4-3' ELSE '4-4-2' END,now(),now())
   RETURNING id INTO v_formation_id;
  END IF;
  IF EXISTS (SELECT 1 FROM public.lineup_players WHERE formation_id=v_formation_id AND deleted_at IS NULL GROUP BY player_id HAVING count(*)>1)
   OR EXISTS (SELECT 1 FROM public.substitutes WHERE formation_id=v_formation_id AND deleted_at IS NULL GROUP BY player_id HAVING count(*)>1)
  THEN RAISE EXCEPTION 'Duplicate formation entries need review'; END IF;

  UPDATE public.lineup_players l SET deleted_at=now(),updated_at=now()
  WHERE formation_id=v_formation_id AND deleted_at IS NULL AND NOT EXISTS (
   SELECT 1 FROM matchday_squad s WHERE s.team_id=team.team_id AND s.starter AND s.player_id=l.player_id
  );
  UPDATE public.substitutes b SET deleted_at=now(),updated_at=now()
  WHERE formation_id=v_formation_id AND deleted_at IS NULL AND NOT EXISTS (
   SELECT 1 FROM matchday_squad s WHERE s.team_id=team.team_id AND NOT s.starter AND s.player_id=b.player_id
  );
  UPDATE public.unavailable_players SET deleted_at=now(),updated_at=now()
  WHERE formation_id=v_formation_id AND deleted_at IS NULL;

  UPDATE public.lineup_players l SET name=p.name,jersey_number=s.shirt,position=s.match_position,x=s.x,y=s.y,updated_at=now()
  FROM matchday_squad s JOIN public.players p ON p.id=s.player_id
  WHERE l.formation_id=v_formation_id AND l.player_id=s.player_id AND l.deleted_at IS NULL
   AND s.team_id=team.team_id AND s.starter
   AND (l.name,l.jersey_number,l.position,l.x,l.y) IS DISTINCT FROM (p.name,s.shirt,s.match_position,s.x,s.y);
  INSERT INTO public.lineup_players(uuid,formation_id,player_id,name,jersey_number,position,x,y,created_at,updated_at)
  SELECT gen_random_uuid(),v_formation_id,p.id,p.name,s.shirt,s.match_position,s.x,s.y,now(),now()
  FROM matchday_squad s JOIN public.players p ON p.id=s.player_id
  WHERE s.team_id=team.team_id AND s.starter AND NOT EXISTS (
   SELECT 1 FROM public.lineup_players l WHERE l.formation_id=v_formation_id AND l.player_id=p.id AND l.deleted_at IS NULL
  ) ORDER BY s.y,s.x;

  UPDATE public.substitutes b SET name=p.name,jersey_number=s.shirt,position=s.match_position,updated_at=now()
  FROM matchday_squad s JOIN public.players p ON p.id=s.player_id
  WHERE b.formation_id=v_formation_id AND b.player_id=s.player_id AND b.deleted_at IS NULL
   AND s.team_id=team.team_id AND NOT s.starter
   AND (b.name,b.jersey_number,b.position) IS DISTINCT FROM (p.name,s.shirt,s.match_position);
  INSERT INTO public.substitutes(uuid,formation_id,player_id,name,jersey_number,position,created_at,updated_at)
  SELECT gen_random_uuid(),v_formation_id,p.id,p.name,s.shirt,s.match_position,now(),now()
  FROM matchday_squad s JOIN public.players p ON p.id=s.player_id
  WHERE s.team_id=team.team_id AND NOT s.starter AND NOT EXISTS (
   SELECT 1 FROM public.substitutes b WHERE b.formation_id=v_formation_id AND b.player_id=p.id AND b.deleted_at IS NULL
  ) ORDER BY s.shirt;

  IF (SELECT count(*) FROM public.lineup_players WHERE formation_id=v_formation_id AND deleted_at IS NULL)<>11
   OR (SELECT count(*) FROM public.substitutes WHERE formation_id=v_formation_id AND deleted_at IS NULL)<>12
   OR EXISTS (SELECT 1 FROM public.lineup_players l JOIN public.substitutes b ON b.player_id=l.player_id AND b.formation_id=l.formation_id
     WHERE l.formation_id=v_formation_id AND l.deleted_at IS NULL AND b.deleted_at IS NULL)
  THEN RAISE EXCEPTION 'Saved formation verification failed'; END IF;
 END LOOP;
END $lineups$;

SELECT f.match_id,c.name,f.formation,
 (SELECT count(*) FROM public.lineup_players l WHERE l.formation_id=f.id AND l.deleted_at IS NULL) AS starting_xi,
 (SELECT count(*) FROM public.substitutes s WHERE s.formation_id=f.id AND s.deleted_at IS NULL) AS substitutes
FROM public.formations f JOIN public.clubs c ON c.id=f.club_id
JOIN public.matches m ON m.id=f.match_id
WHERE f.deleted_at IS NULL AND f.club_id IN(13,37) AND m.home_club_id=13 AND m.away_club_id=37
 AND (m.date AT TIME ZONE 'UTC')::date=DATE '2026-11-14'
ORDER BY f.club_id;
COMMIT;
