-- Profile enrichment for the 46 imported matchday players.
-- Researched 16 November 2026. Uganda's 23 profiles are the priority.
-- Birth dates follow CAF registration data where profiles disagree.
-- Amogelang Reetsang's 9 August 2001 birth date was supplied by the user.
-- Unknown Botswana club numbers and Larona's missing portrait remain unknown.
-- Club and national shirt numbers are stored separately.
-- Bulk call-ups are attributed to the first active platform administrator.
-- Re-running this file updates the same player IDs and preserves active call-ups.
BEGIN;
SET LOCAL lock_timeout='10s';
SET LOCAL statement_timeout='60s';
SELECT pg_advisory_xact_lock(hashtextextended('afrigoals-matchday-player-import',0));
LOCK TABLE public.players,public.national_callups IN SHARE ROW EXCLUSIVE MODE;
CREATE TEMP TABLE player_enrichment (
 player_id bigint PRIMARY KEY,expected_name text NOT NULL,team_id bigint NOT NULL,
 national_shirt integer NOT NULL,expected_club text NOT NULL,birth_date date NOT NULL,
 player_position text NOT NULL,club_shirt integer,photo_url text,sources jsonb NOT NULL
) ON COMMIT DROP;
INSERT INTO player_enrichment VALUES
 (200, 'David Lukwago', 13, 1, 'VILLA SC', '2002-10-10', 'Goalkeeper', 18, 'https://scvilla.co.ug/wp-content/uploads/2026/09/WhatsApp-Image-2026-10-20-at-10.48-150x150.jpeg', '{"birth_date": "https://www.fotmob.com/players/2272850", "position": "https://www.fotmob.com/players/2272850", "club_shirt": "https://scvilla.co.ug/players-and-staff/senior-team/", "photo_url": "https://scvilla.co.ug/wp-content/uploads/2026/09/WhatsApp-Image-2026-10-20-at-10.48-150x150.jpeg"}'::jsonb),
 (412, 'Nathan Asiimwe', 13, 2, 'Salford City FC', '2004-12-29', 'Right Back', 2, 'https://images.fotmob.com/image_resources/playerimages/1420829.png', '{"birth_date": "https://www.fotmob.com/players/1420829", "position": "https://www.fotmob.com/players/1420829", "club_shirt": "https://www.fotmob.com/players/1420829", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1420829.png"}'::jsonb),
 (413, 'Rogers Torach', 13, 3, 'CS Constantine', '2003-06-23', 'Center Back', 24, 'https://images.fotmob.com/image_resources/playerimages/1453717.png', '{"birth_date": "https://www.fotmob.com/players/1453717", "position": "https://www.fotmob.com/players/1453717", "club_shirt": "https://african-football.com/matchs/football/constantine-aso-chlef-12364/compos/", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1453717.png"}'::jsonb),
 (414, 'Kenneth Ssemakula', 13, 4, 'Al Ittihad Alexandria Club', '2002-11-04', 'Defensive Midfielder', 44, 'https://images.fotmob.com/image_resources/playerimages/1294324.png', '{"birth_date": "https://www.fotmob.com/players/1294324", "position": "https://www.fotmob.com/players/1294324", "club_shirt": "https://www.fotmob.com/players/1294324", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1294324.png"}'::jsonb),
 (415, 'Toby Sibbick', 13, 5, 'Burton Albion FC', '1999-05-23', 'Center Back', 6, 'https://images.fotmob.com/image_resources/playerimages/821400.png', '{"birth_date": "https://www.fotmob.com/players/821400", "position": "https://www.fotmob.com/players/821400", "club_shirt": "https://www.fotmob.com/players/821400", "photo_url": "https://images.fotmob.com/image_resources/playerimages/821400.png"}'::jsonb),
 (416, 'Bobosi Byaruhanga', 13, 6, 'Oakland Roots SC', '2001-12-03', 'Midfielder', 6, 'https://images.fotmob.com/image_resources/playerimages/1242220.png', '{"birth_date": "https://www.fotmob.com/players/1242220", "position": "https://www.fotmob.com/players/1242220", "club_shirt": "https://www.fotmob.com/players/1242220", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1242220.png"}'::jsonb),
 (417, 'Sadam Masereka', 13, 7, 'Colorado Springs Switchbacks FC', '2004-09-09', 'Winger', 17, 'https://images.fotmob.com/image_resources/playerimages/1886516.png', '{"birth_date": "https://www.fotmob.com/players/1886516", "position": "https://www.fotmob.com/players/1886516", "club_shirt": "https://www.fotmob.com/players/1886516", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1886516.png"}'::jsonb),
 (418, 'Khalid Aucho', 13, 8, 'Sidama Bunna', '1993-08-08', 'Midfielder', 8, 'https://images.fotmob.com/image_resources/playerimages/429454.png', '{"birth_date": "https://www.fotmob.com/players/429454", "position": "https://www.fotmob.com/players/429454", "club_shirt": "https://sports.bzzoiro.com/teams/2740/", "photo_url": "https://images.fotmob.com/image_resources/playerimages/429454.png"}'::jsonb),
 (419, 'Arnold Vula', 13, 9, 'K. Beerschot V.A.', '1999-03-26', 'Striker', 15, 'https://images.fotmob.com/image_resources/playerimages/1552558.png', '{"birth_date": "https://www.fotmob.com/players/1552558", "position": "https://www.fotmob.com/players/1552558", "club_shirt": "https://www.fotmob.com/players/1552558", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1552558.png"}'::jsonb),
 (420, 'Travis Mutyaba', 13, 10, 'CS Sfaxien', '2005-08-07', 'Attacking Midfielder', 12, 'https://images.fotmob.com/image_resources/playerimages/1328313.png', '{"birth_date": "https://www.fotmob.com/players/1328313", "position": "https://www.fotmob.com/players/1328313", "club_shirt": "https://www.aiscore.com/player-t-mutyaba/wv78xi68nlrb2kr", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1328313.png"}'::jsonb),
 (421, 'Herbert Bockhorn', 13, 11, 'FC Magdeburg', '1995-01-31', 'Right Back', 7, 'https://images.fotmob.com/image_resources/playerimages/866164.png', '{"birth_date": "https://www.fotmob.com/players/866164", "position": "https://www.fotmob.com/players/866164", "club_shirt": "https://www.fotmob.com/players/866164", "photo_url": "https://images.fotmob.com/image_resources/playerimages/866164.png"}'::jsonb),
 (422, 'Kelvin Boateng', 13, 13, 'FK Austria Wien', '2000-03-24', 'Striker', 14, 'https://images.fotmob.com/image_resources/playerimages/1160111.png', '{"birth_date": "https://www.fotmob.com/players/1160111", "position": "https://www.fotmob.com/players/1160111", "club_shirt": "https://www.fotmob.com/players/1160111", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1160111.png"}'::jsonb),
 (423, 'Denis Omedi', 13, 14, 'Gor Mahia FC', '1994-06-13', 'Forward', 14, 'https://images.fotmob.com/image_resources/playerimages/1582279.png', '{"birth_date": "https://www.fotmob.com/players/1582279", "position": "https://www.fotmob.com/players/1582279", "club_shirt": "https://kickoffscore.com/players/445223/denis-omedi", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1582279.png"}'::jsonb),
 (424, 'Baba Alhassan', 13, 15, 'Mardin 1969 Spor', '2000-01-03', 'Defensive Midfielder', 42, 'https://images.fotmob.com/image_resources/playerimages/1197337.png', '{"birth_date": "https://www.fotmob.com/players/1197337", "position": "https://www.fotmob.com/players/1197337", "club_shirt": "https://www.fotmob.com/players/1197337", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1197337.png"}'::jsonb),
 (93, 'Hilary Mukundane', 13, 16, 'Vipers Sc', '2001-12-09', 'Defender', 16, 'https://images.fotmob.com/image_resources/playerimages/1792307.png', '{"birth_date": "https://www.cafonline.com/media/3ovhb4zn/uganda.pdf", "position": "https://www.fotmob.com/players/1792307", "club_shirt": "https://viperssc.co.ug/player/hillary-mukundane/", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1792307.png"}'::jsonb),
 (425, 'James Bogere', 13, 17, 'AGF', '2008-02-02', 'Striker', 28, 'https://images.fotmob.com/image_resources/playerimages/1843410.png', '{"birth_date": "https://www.fotmob.com/players/1843410", "position": "https://www.fotmob.com/players/1843410", "club_shirt": "https://www.fotmob.com/players/1843410", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1843410.png"}'::jsonb),
 (426, 'Denis Onyango', 13, 18, 'Mamelodi Sundowns FC', '1985-05-15', 'Goalkeeper', 1, 'https://images.fotmob.com/image_resources/playerimages/114326.png', '{"birth_date": "https://www.fotmob.com/players/114326", "position": "https://www.fotmob.com/players/114326", "club_shirt": "https://www.fotmob.com/players/114326", "photo_url": "https://images.fotmob.com/image_resources/playerimages/114326.png"}'::jsonb),
 (427, 'Kingdom Osayi', 13, 19, 'Assyriska FF', '2004-06-03', 'Goalkeeper', 31, 'https://images.fotmob.com/image_resources/playerimages/1763375.png', '{"birth_date": "https://www.fotmob.com/players/1763375", "position": "https://www.fotmob.com/players/1763375", "club_shirt": "https://www.fotmob.com/players/1763375", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1763375.png"}'::jsonb),
 (428, 'Toto Majub', 13, 20, 'Atlanta United FC', '2006-06-03', 'Center Back', 41, 'https://images.fotmob.com/image_resources/playerimages/1781198.png', '{"birth_date": "https://www.fotmob.com/players/1781198", "position": "https://www.fotmob.com/players/1781198", "club_shirt": "https://www.fotmob.com/players/1781198", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1781198.png"}'::jsonb),
 (260, 'Allan Okello', 13, 21, 'AS Far Rabat', '2000-07-04', 'Winger', 14, 'https://images.fotmob.com/image_resources/playerimages/938400.png', '{"birth_date": "https://www.fotmob.com/players/938400", "position": "https://www.fotmob.com/players/938400", "club_shirt": "https://www.fotmob.com/players/938400", "photo_url": "https://images.fotmob.com/image_resources/playerimages/938400.png"}'::jsonb),
 (429, 'Allan Oyirwoth', 13, 22, 'Dunfermline FC', '2007-01-23', 'Midfielder', 19, 'https://images.fotmob.com/image_resources/playerimages/1584040.png', '{"birth_date": "https://www.fotmob.com/players/1584040", "position": "https://www.fotmob.com/players/1584040", "club_shirt": "https://www.fotmob.com/players/1584040", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1584040.png"}'::jsonb),
 (430, 'Aziz Kayondo', 13, 23, 'FC Slovan Liberec', '2002-10-06', 'Left Back', 27, 'https://images.fotmob.com/image_resources/playerimages/1293353.png', '{"birth_date": "https://www.fotmob.com/players/1293353", "position": "https://www.fotmob.com/players/1293353", "club_shirt": "https://www.fotmob.com/players/1293353", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1293353.png"}'::jsonb),
 (205, 'Frank Ssebufu', 13, 24, 'VILLA SC', '2001-09-09', 'Forward', 9, 'https://images.fotmob.com/image_resources/playerimages/1496201.png', '{"birth_date": "https://www.fotmob.com/players/1496201", "position": "https://www.fotmob.com/players/1496201", "club_shirt": "https://scvilla.co.ug/players-and-staff/senior-team/", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1496201.png"}'::jsonb),
 (389, 'Victor James', 37, 1, 'CAPS United', '2002-08-27', 'Goalkeeper', NULL, 'https://images.fotmob.com/image_resources/playerimages/1648413.png', '{"birth_date": "https://www.fotmob.com/players/1648413", "position": "https://www.fotmob.com/players/1648413", "club_shirt": "Not verified", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1648413.png"}'::jsonb),
 (390, 'Botsile Sakana', 37, 2, 'Jwaneng Galaxy', '2001-01-26', 'Center Back', NULL, 'https://images.fotmob.com/image_resources/playerimages/1648418.png', '{"birth_date": "https://www.fotmob.com/players/1648418", "position": "https://www.fotmob.com/players/1648418", "club_shirt": "Not verified", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1648418.png"}'::jsonb),
 (391, 'Larona Modisaemang', 37, 3, 'Jwaneng Galaxy', '2002-01-29', 'Defender', 29, NULL, '{"birth_date": "https://www.fotmob.com/players/2458239", "position": "https://www.fotmob.com/players/2458239", "club_shirt": "https://www.transfermarkt.com/jwaneng-galaxy-fc/kader/verein/52369", "photo_url": "No verified portrait found"}'::jsonb),
 (392, 'Mosha Gaolaolwe', 37, 4, 'Mochudi Centre Chiefs', '1993-12-25', 'Center Back', 4, 'https://images.fotmob.com/image_resources/playerimages/579664.png', '{"birth_date": "https://www.fotmob.com/players/579664", "position": "https://www.fotmob.com/players/579664", "club_shirt": "https://www.sofascore.com/football/player/mosha-gaolaolwe/932978", "photo_url": "https://images.fotmob.com/image_resources/playerimages/579664.png"}'::jsonb),
 (393, 'Alford Velaphi', 37, 5, 'Gaborone United', '1999-01-18', 'Right Back', 4, 'https://images.fotmob.com/image_resources/playerimages/1034039.png', '{"birth_date": "https://www.fotmob.com/players/1034039", "position": "https://www.fotmob.com/players/1034039", "club_shirt": "https://www.fotmob.com/players/1034039", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1034039.png"}'::jsonb),
 (394, 'Gape Mohutsiwa', 37, 6, 'Ittihad Tanger', '1997-03-20', 'Midfielder', 8, 'https://images.fotmob.com/image_resources/playerimages/938391.png', '{"birth_date": "https://www.fotmob.com/players/938391", "position": "https://www.fotmob.com/players/938391", "club_shirt": "https://www.fotmob.com/players/938391", "photo_url": "https://images.fotmob.com/image_resources/playerimages/938391.png"}'::jsonb),
 (395, 'Losika Ratshukudu', 37, 7, 'NK Slaven Belupo', '2006-02-01', 'Striker', 14, 'https://images.fotmob.com/image_resources/playerimages/1870145.png', '{"birth_date": "https://www.fotmob.com/players/1870145", "position": "https://www.fotmob.com/players/1870145", "club_shirt": "https://www.fotmob.com/players/1870145", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1870145.png"}'::jsonb),
 (396, 'Amogelang Reetsang', 37, 8, 'Orapa United', '2001-08-09', 'Midfielder', NULL, 'https://www.flashscore.com/res/image/data/bTh5yUCr-jsf4t6jt.png', '{"birth_date": "Provided by the administrator in this chat on 2026-11-16", "position": "https://www.fotmob.com/players/2458285", "club_shirt": "Not verified", "photo_url": "https://www.flashscore.com/res/image/data/bTh5yUCr-jsf4t6jt.png"}'::jsonb),
 (397, 'Godfrey Tauyatswala', 37, 9, 'Jwaneng Galaxy', '2001-01-25', 'Forward', NULL, 'https://www.flashscore.com/res/image/data/thE25nZA-E18J9Krj.png', '{"birth_date": "https://www.fotmob.com/players/1730192", "position": "https://www.fotmob.com/players/1730192", "club_shirt": "Not verified", "photo_url": "https://www.flashscore.com/res/image/data/thE25nZA-E18J9Krj.png"}'::jsonb),
 (398, 'Thero Setsile', 37, 10, 'Tonota FC', '1995-10-08', 'Forward', 10, 'https://images.fotmob.com/image_resources/playerimages/854038.png', '{"birth_date": "https://www.fotmob.com/players/854038", "position": "https://www.fotmob.com/players/854038", "club_shirt": "https://www.tonotafc.com/player/thero-setsile/", "photo_url": "https://images.fotmob.com/image_resources/playerimages/854038.png"}'::jsonb),
 (399, 'Tumisang Orebonye', 37, 11, 'Hassania Agadir', '1996-03-26', 'Striker', 28, 'https://images.fotmob.com/image_resources/playerimages/854042.png', '{"birth_date": "https://www.fotmob.com/players/854042", "position": "https://www.fotmob.com/players/854042", "club_shirt": "https://www.fotmob.com/players/854042", "photo_url": "https://images.fotmob.com/image_resources/playerimages/854042.png"}'::jsonb),
 (400, 'Thatayaone Kgamanyane', 37, 12, 'Gaborone United', '1996-01-30', 'Forward', 10, 'https://images.fotmob.com/image_resources/playerimages/854041.png', '{"birth_date": "https://www.fotmob.com/players/854041", "position": "https://www.fotmob.com/players/854041", "club_shirt": "https://www.fotmob.com/players/854041", "photo_url": "https://images.fotmob.com/image_resources/playerimages/854041.png"}'::jsonb),
 (401, 'Kutlwelo Mpolokang', 37, 13, 'Jwaneng Galaxy', '1996-02-09', 'Midfielder', 6, 'https://img.sofascore.com/api/v1/player/1111002/image', '{"birth_date": "https://www.fotmob.com/players/970158", "position": "https://www.fotmob.com/players/970158", "club_shirt": "https://www.fotmob.com/players/970158", "photo_url": "https://img.sofascore.com/api/v1/player/1111002/image"}'::jsonb),
 (402, 'Godiraone Modingwane', 37, 14, 'Botswana Defence Force XI', '1996-06-25', 'Midfielder', 14, 'https://images.fotmob.com/image_resources/playerimages/941962.png', '{"birth_date": "https://www.cafonline.com/media/n0gn3co4/botswana.pdf", "position": "https://www.fotmob.com/players/941962", "club_shirt": "https://www.sofascore.com/football/player/godairone-modingwane/1127661", "photo_url": "https://images.fotmob.com/image_resources/playerimages/941962.png"}'::jsonb),
 (403, 'Omphile Ramoagi', 37, 15, 'Gaborone United', '2004-01-03', 'Midfielder', 3, 'https://www.flashscore.com/res/image/data/pCFgY4Cr-zmF7DYDK.png', '{"birth_date": "https://www.fotmob.com/players/1870123", "position": "https://www.fotmob.com/players/1870123", "club_shirt": "https://www.365scores.com/football/player/omphile-ramoagi-228803", "photo_url": "https://www.flashscore.com/res/image/data/pCFgY4Cr-zmF7DYDK.png"}'::jsonb),
 (404, 'Kenneth Mmoko', 37, 16, 'Sua Flamingoes', '1999-10-13', 'Goalkeeper', NULL, 'https://pbs.twimg.com/media/GQ6HsutXUAAIcQ9.jpg', '{"birth_date": "https://www.fotmob.com/players/1678263", "position": "https://www.fotmob.com/players/1678263", "club_shirt": "Not verified", "photo_url": "https://pbs.twimg.com/media/GQ6HsutXUAAIcQ9.jpg"}'::jsonb),
 (405, 'Mbatshi Elias', 37, 17, 'Orapa United', '1995-07-10', 'Forward', 9, 'https://orapaunitedfc.com/wp-content/uploads/2021/01/MbatshiElias-445x400.png', '{"birth_date": "https://www.fotmob.com/players/1090186", "position": "https://www.fotmob.com/players/1090186", "club_shirt": "https://orapaunitedfc.com/player/mbatshi-chida-elias/", "photo_url": "https://orapaunitedfc.com/wp-content/uploads/2021/01/MbatshiElias-445x400.png"}'::jsonb),
 (406, 'Lebogang Ditsele', 37, 18, 'Gaborone United', '1996-04-20', 'Defensive Midfielder', 18, 'https://img.sofascore.com/api/v1/player/990862/image', '{"birth_date": "https://www.fotmob.com/players/598966", "position": "https://www.fotmob.com/players/598966", "club_shirt": "https://www.sofascore.com/football/player/lebogang-ditsele/990862", "photo_url": "https://img.sofascore.com/api/v1/player/990862/image"}'::jsonb),
 (407, 'Chicco Molefe', 37, 19, 'Jwaneng Galaxy', '1995-07-29', 'Defender', 19, 'https://images.fotmob.com/image_resources/playerimages/1693148.png', '{"birth_date": "https://www.fotmob.com/players/1693148", "position": "https://www.fotmob.com/players/1693148", "club_shirt": "https://www.transfermarkt.com/jwaneng-galaxy-fc/kader/verein/52369", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1693148.png"}'::jsonb),
 (408, 'Tebogo Kopelang', 37, 20, 'Jwaneng Galaxy', '2002-04-24', 'Center Back', 20, 'https://images.fotmob.com/image_resources/playerimages/1371208.png', '{"birth_date": "https://www.fotmob.com/players/1371208", "position": "https://www.fotmob.com/players/1371208", "club_shirt": "https://www.zerozero.pt/jogo/2026-02-28-bdf-xi-jwaneng-galaxy/11354426", "photo_url": "https://images.fotmob.com/image_resources/playerimages/1371208.png"}'::jsonb),
 (409, 'Thabo Maponda', 37, 21, 'Gaborone United', '1997-07-17', 'Forward', 21, 'https://img.sofascore.com/api/v1/player/1605682/image', '{"birth_date": "https://www.fotmob.com/players/1581339", "position": "https://www.fotmob.com/players/1581339", "club_shirt": "https://www.sofascore.com/football/player/thabo-maponda/1605682", "photo_url": "https://img.sofascore.com/api/v1/player/1605682/image"}'::jsonb),
 (410, 'Gilbert Baruti', 37, 22, 'Mochudi Centre Chiefs', '1992-03-16', 'Attacking Midfielder', 9, 'https://images.fotmob.com/image_resources/playerimages/941965.png', '{"birth_date": "https://www.fotmob.com/players/941965", "position": "https://www.fotmob.com/players/941965", "club_shirt": "https://www.sofascore.com/football/player/gilbert-baruti/990612", "photo_url": "https://images.fotmob.com/image_resources/playerimages/941965.png"}'::jsonb),
 (411, 'Goitseone Phoko', 37, 23, 'Siwelele FC', '1994-12-13', 'Goalkeeper', 31, 'https://images.fotmob.com/image_resources/playerimages/686288.png', '{"birth_date": "https://www.fotmob.com/players/686288", "position": "https://www.fotmob.com/players/686288", "club_shirt": "https://www.fotmob.com/players/686288", "photo_url": "https://images.fotmob.com/image_resources/playerimages/686288.png"}'::jsonb);

DO $validate$
BEGIN
 IF (SELECT count(*) FROM player_enrichment)<>46 OR
    (SELECT count(*) FROM player_enrichment e JOIN public.players p ON p.id=e.player_id
     JOIN public.clubs c ON c.id=p.club_id
     WHERE p.deleted_at IS NULL AND p.name=e.expected_name AND p.national_team_id=e.team_id
       AND p.national_jersey_number=e.national_shirt AND c.name=e.expected_club)<>46 THEN
   RAISE EXCEPTION 'The imported player identities or registrations have changed. Review before updating.';
 END IF;
 IF EXISTS (
   SELECT 1 FROM player_enrichment e JOIN public.players p ON p.id=e.player_id
   JOIN public.players other ON other.club_id=p.club_id AND other.id<>p.id AND other.deleted_at IS NULL
   WHERE e.club_shirt>0 AND other.jersey_number=e.club_shirt
     AND p.jersey_number IS DISTINCT FROM e.club_shirt
 ) THEN
   RAISE EXCEPTION 'A researched club shirt number is already assigned to another player. Review before updating.';
 END IF;
 IF EXISTS (
   SELECT 1 FROM player_enrichment e JOIN public.players p ON p.id=e.player_id
   WHERE e.club_shirt>0 GROUP BY p.club_id,e.club_shirt HAVING count(*)>1
 ) THEN
   RAISE EXCEPTION 'The staged club shirt numbers conflict';
 END IF;
END $validate$;

UPDATE public.players p SET
 date_of_birth=e.birth_date::timestamp AT TIME ZONE 'UTC',
 position=e.player_position,
 jersey_number=COALESCE(e.club_shirt,p.jersey_number),
 photo_url=COALESCE(e.photo_url,p.photo_url),
 updated_at=now()
FROM player_enrichment e
WHERE p.id=e.player_id AND (
 p.date_of_birth IS DISTINCT FROM (e.birth_date::timestamp AT TIME ZONE 'UTC')
 OR p.position IS DISTINCT FROM e.player_position
 OR p.jersey_number IS DISTINCT FROM COALESCE(e.club_shirt,p.jersey_number)
 OR p.photo_url IS DISTINCT FROM COALESCE(e.photo_url,p.photo_url)
);

DO $callups$
DECLARE v_actor bigint;
BEGIN
 SELECT id INTO STRICT v_actor FROM public.users
 WHERE role='afrigoals_admin' AND deleted_at IS NULL ORDER BY id LIMIT 1;
 INSERT INTO public.national_callups(team_id,player_id,called_by,called_at,jersey_number)
 SELECT team_id,player_id,v_actor,now(),national_shirt FROM player_enrichment
 ON CONFLICT(team_id,player_id) WHERE withdrawn_at IS NULL DO NOTHING;
END $callups$;

SELECT c.name AS national_team,count(*) AS listed_players,
 count(*) FILTER (WHERE p.date_of_birth IS NOT NULL) AS birth_dates,
 count(*) FILTER (WHERE COALESCE(p.photo_url,'')<>'') AS photos,
 count(*) FILTER (WHERE p.jersey_number>0) AS club_numbers,
 count(*) FILTER (WHERE COALESCE(p.position,'')<>'') AS positions,
 count(*) FILTER (WHERE n.id IS NOT NULL) AS called_up
FROM player_enrichment e JOIN public.players p ON p.id=e.player_id
JOIN public.clubs c ON c.id=e.team_id
LEFT JOIN public.national_callups n ON n.team_id=e.team_id AND n.player_id=e.player_id AND n.withdrawn_at IS NULL
GROUP BY c.name ORDER BY c.name;

COMMIT;
