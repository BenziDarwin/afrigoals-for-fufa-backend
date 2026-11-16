-- User-supplied matchday player list for Uganda v Botswana, 14 November 2026.
-- Club affiliations researched 16 November 2026. Each row records its source.
-- Botswana club reports differ for Botsile Sakana. The 9 November squad report
-- lists Jwaneng Galaxy. Kutlwelo Mpolokang is linked to Galaxy on a reported loan.
-- National shirt numbers come from the supplied matchday list. Club shirt numbers
-- and birth dates are left unknown for newly created profiles.
-- This data import can be rerun. It reuses active profiles and club records.
-- Run the complete file in a PostgreSQL session. It commits only after validation.

BEGIN;
SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '60s';
SELECT pg_advisory_xact_lock(hashtextextended('afrigoals-matchday-player-import', 0));
LOCK TABLE public.clubs, public.players IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE import_roster (
    team_code text NOT NULL,
    shirt integer NOT NULL,
    player_name text NOT NULL,
    aliases text[] NOT NULL,
    player_position text NOT NULL,
    club_name text NOT NULL,
    club_country text NOT NULL,
    club_aliases text[] NOT NULL,
    club_source text NOT NULL,
    PRIMARY KEY (team_code, shirt),
    UNIQUE (team_code, player_name)
) ON COMMIT DROP;

INSERT INTO import_roster VALUES
    ('UG', 18, 'Denis Onyango', ARRAY[]::text[], 'Goalkeeper', 'Mamelodi Sundowns FC', 'ZA', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 4, 'Kenneth Ssemakula', ARRAY['Kenneth Semakula']::text[], 'Defender', 'Al Ittihad Alexandria Club', 'EG', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 3, 'Rogers Torach', ARRAY['Rogers Torach Ochaki']::text[], 'Defender', 'CS Constantine', 'DZ', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 5, 'Toby Sibbick', ARRAY[]::text[], 'Defender', 'Burton Albion FC', 'GB', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 2, 'Nathan Asiimwe', ARRAY[]::text[], 'Defender', 'Salford City FC', 'GB', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 15, 'Baba Alhassan', ARRAY[]::text[], 'Midfielder', 'Mardin 1969 Spor', 'TR', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 6, 'Bobosi Byaruhanga', ARRAY[]::text[], 'Midfielder', 'Oakland Roots SC', 'US', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 23, 'Aziz Kayondo', ARRAY['Aziizi Kayondo','Abdu Aziizi Kayondo','Abdul Aziz Kayondo']::text[], 'Defender', 'FC Slovan Liberec', 'CZ', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 13, 'Kelvin Boateng', ARRAY['Kelvin Owusu Boateng']::text[], 'Forward', 'FK Austria Wien', 'AT', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 9, 'Arnold Vula', ARRAY['Arnold Vula Lamb Luth']::text[], 'Forward', 'K. Beerschot V.A.', 'BE', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 17, 'James Bogere', ARRAY[]::text[], 'Forward', 'AGF', 'DK', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 1, 'David Lukwago', ARRAY[]::text[], 'Goalkeeper', 'VILLA SC', 'UG', ARRAY['SC Villa']::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 7, 'Sadam Masereka', ARRAY['Saddam Masereka']::text[], 'Forward', 'Colorado Springs Switchbacks FC', 'US', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 8, 'Khalid Aucho', ARRAY[]::text[], 'Midfielder', 'Sidama Bunna', 'ET', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 10, 'Travis Mutyaba', ARRAY[]::text[], 'Forward', 'CS Sfaxien', 'TN', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 11, 'Herbert Bockhorn', ARRAY['Herbert Bochorn']::text[], 'Defender', 'FC Magdeburg', 'DE', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 14, 'Denis Omedi', ARRAY[]::text[], 'Forward', 'Gor Mahia FC', 'KE', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 16, 'Hilary Mukundane', ARRAY[]::text[], 'Defender', 'Vipers Sc', 'UG', ARRAY['Vipers SC']::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 19, 'Kingdom Osayi', ARRAY[]::text[], 'Goalkeeper', 'Assyriska FF', 'SE', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 20, 'Toto Majub', ARRAY['Majub Toto','Brian Toto Majub']::text[], 'Defender', 'Atlanta United FC', 'US', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 21, 'Allan Okello', ARRAY[]::text[], 'Forward', 'AS Far Rabat', 'MA', ARRAY['AS FAR','AS FAR Rabat']::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 22, 'Allan Oyirwoth', ARRAY[]::text[], 'Midfielder', 'Dunfermline FC', 'GB', ARRAY[]::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('UG', 24, 'Frank Ssebuufu', ARRAY['Frank Ssebufu']::text[], 'Forward', 'VILLA SC', 'UG', ARRAY['SC Villa']::text[], 'https://fufa.co.ug/totalenergies-afcon-2027-qualifiers-paul-put-names-squad-as-captain-aucho-prepares-for-cranes-farewell/'),
    ('BW', 23, 'Goitseone Phoko', ARRAY[]::text[], 'Goalkeeper', 'Siwelele FC', 'ZA', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 5, 'Alford Velaphi', ARRAY[]::text[], 'Defender', 'Gaborone United', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 20, 'Tebogo Kopelang', ARRAY[]::text[], 'Defender', 'Jwaneng Galaxy', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 4, 'Mosha Gaolaolwe', ARRAY['Mosa Gaolaolwe']::text[], 'Defender', 'Mochudi Centre Chiefs', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 19, 'Chicco Molefe', ARRAY[]::text[], 'Defender', 'Jwaneng Galaxy', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 7, 'Losika Ratshukudu', ARRAY[]::text[], 'Midfielder', 'NK Slaven Belupo', 'HR', ARRAY[]::text[], 'https://semafor.hns.family/igraci/1233684/losika-ratshukudu/'),
    ('BW', 6, 'Gape Mohutsiwa', ARRAY['Gape Edwin Mohutsiwa']::text[], 'Midfielder', 'Ittihad Tanger', 'MA', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 14, 'Godiraone Modingwane', ARRAY['Godairone Modingwane','Godarione Modingwane']::text[], 'Midfielder', 'Botswana Defence Force XI', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 10, 'Thero Setsile', ARRAY['Francis Thero Setsile']::text[], 'Midfielder', 'Tonota FC', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 22, 'Gilbert Baruti', ARRAY[]::text[], 'Forward', 'Mochudi Centre Chiefs', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 11, 'Tumisang Orebonye', ARRAY[]::text[], 'Forward', 'Hassania Agadir', 'MA', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 1, 'Victor James', ARRAY[]::text[], 'Goalkeeper', 'CAPS United', 'ZW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 2, 'Botsile Sakana', ARRAY['Botsile Tiroyaone Sakana']::text[], 'Defender', 'Jwaneng Galaxy', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 3, 'Larona Modisaemang', ARRAY[]::text[], 'Defender', 'Jwaneng Galaxy', 'BW', ARRAY[]::text[], 'https://dailynews.gov.bw/news-detail/90592'),
    ('BW', 8, 'Amogelang Reetsang', ARRAY[]::text[], 'Midfielder', 'Orapa United', 'BW', ARRAY[]::text[], 'https://www.liveresult.ru/football/players/Amogelang-Reetsang'),
    ('BW', 9, 'Godfrey Tauyatswala', ARRAY['Godfrey Nonko Tauyatswala']::text[], 'Forward', 'Jwaneng Galaxy', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 12, 'Thatayaone Kgamanyane', ARRAY['Thatayaone Witness Kgamanyane']::text[], 'Forward', 'Gaborone United', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 13, 'Kutlwelo Mpolokang', ARRAY['Kutwelo Mpolokang','Kutlwelo William Mpolokang']::text[], 'Midfielder', 'Jwaneng Galaxy', 'BW', ARRAY[]::text[], 'https://www.lemauricien.com/sports/aspl-2000-jwaneng-galaxy-reussir-son-retour-en-afrique/716731/'),
    ('BW', 15, 'Omphile Ramoagi', ARRAY['Omphile Ramogi']::text[], 'Midfielder', 'Gaborone United', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 16, 'Kenneth Mmoko', ARRAY['Kenneth Moko']::text[], 'Goalkeeper', 'Sua Flamingoes', 'BW', ARRAY[]::text[], 'https://dailynews.gov.bw/common_up/dailynews/dailynews_pdf/10-11-2026_07-43-31_1794289411_dailynews_pdf.pdf'),
    ('BW', 17, 'Mbatshi Elias', ARRAY['Mbatshi elias','Elias Mbatshi','Mbatshi Chida Elias']::text[], 'Forward', 'Orapa United', 'BW', ARRAY[]::text[], 'https://orapaunitedfc.com/team/orapa-united-fc/'),
    ('BW', 18, 'Lebogang Ditsele', ARRAY['Lebogang Ditsile']::text[], 'Midfielder', 'Gaborone United', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers'),
    ('BW', 21, 'Thabo Maponda', ARRAY[]::text[], 'Forward', 'Gaborone United', 'BW', ARRAY[]::text[], 'https://supersport.com/football/news/7675ecca-dfdc-4331-8077-2a85a9534f3c/botswana-choose-squad-for-afcon-qualifiers');

CREATE OR REPLACE FUNCTION pg_temp.import_name_key(value text) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
    SELECT lower(regexp_replace(btrim(value), '[^[:alnum:]]', '', 'g'))
$$;

CREATE TEMP TABLE import_teams (
    team_code text PRIMARY KEY,
    team_id bigint NOT NULL
) ON COMMIT DROP;
CREATE TEMP TABLE import_results (
    team_code text NOT NULL,
    supplied_name text NOT NULL,
    player_id bigint NOT NULL UNIQUE,
    was_created boolean NOT NULL,
    club_id bigint NOT NULL
) ON COMMIT DROP;

DO $import$
DECLARE
    r record;
    v_team_name text;
    v_country_name text;
    v_club_country_name text;
    v_team_id bigint;
    v_club_id bigint;
    v_player public.players%ROWTYPE;
    v_ids bigint[];
    v_created boolean;
BEGIN
    IF (SELECT count(*) FROM import_roster) <> 46 THEN
        RAISE EXCEPTION 'Expected 46 supplied players';
    END IF;
    FOR r IN SELECT DISTINCT team_code FROM import_roster ORDER BY team_code LOOP
        SELECT name INTO STRICT v_country_name FROM public.countries WHERE code=r.team_code;
        v_team_name := CASE r.team_code WHEN 'UG' THEN 'Uganda Cranes' WHEN 'BW' THEN 'Botswana Zebras' END;
        SELECT array_agg(id ORDER BY id) INTO v_ids FROM public.clubs
        WHERE deleted_at IS NULL AND (
            (team_type='national' AND country_code=r.team_code)
            OR pg_temp.import_name_key(name)=pg_temp.import_name_key(v_team_name)
        );
        IF cardinality(v_ids)>1 THEN
            RAISE EXCEPTION 'Multiple national team candidates for %: %', r.team_code,v_ids;
        END IF;
        IF cardinality(v_ids)=1 THEN
            v_team_id := v_ids[1];
            UPDATE public.clubs SET team_type='national',country=v_country_name,
                country_code=r.team_code,updated_at=now()
            WHERE id=v_team_id AND (team_type<>'national' OR country_code IS DISTINCT FROM r.team_code);
        ELSE
            INSERT INTO public.clubs(uuid,name,team_type,country,country_code,is_external,created_at,updated_at)
            VALUES(gen_random_uuid(),v_team_name,'national',v_country_name,r.team_code,false,now(),now())
            RETURNING id INTO v_team_id;
        END IF;
        INSERT INTO import_teams VALUES(r.team_code,v_team_id);
    END LOOP;

    FOR r IN SELECT * FROM import_roster ORDER BY team_code,shirt LOOP
        SELECT name INTO STRICT v_country_name FROM public.countries WHERE code=r.team_code;
        SELECT name INTO STRICT v_club_country_name FROM public.countries WHERE code=r.club_country;
        SELECT team_id INTO STRICT v_team_id FROM import_teams WHERE team_code=r.team_code;

        SELECT array_agg(c.id ORDER BY c.id) INTO v_ids FROM public.clubs c
        WHERE c.deleted_at IS NULL AND c.team_type='club'
          AND (c.country_code=r.club_country OR c.country_code IS NULL)
          AND EXISTS (
              SELECT 1 FROM unnest(r.club_aliases || ARRAY[r.club_name]) n(name)
              WHERE pg_temp.import_name_key(c.name)=pg_temp.import_name_key(n.name)
          );
        IF cardinality(v_ids)>1 THEN
            RAISE EXCEPTION 'Multiple club candidates for %: %',r.club_name,v_ids;
        END IF;
        IF cardinality(v_ids)=1 THEN
            v_club_id := v_ids[1];
            UPDATE public.clubs SET country=v_club_country_name,country_code=r.club_country,updated_at=now()
            WHERE id=v_club_id AND country_code IS NULL;
        ELSE
            INSERT INTO public.clubs(uuid,name,team_type,country,country_code,is_external,created_at,updated_at)
            VALUES(gen_random_uuid(),r.club_name,'club',v_club_country_name,r.club_country,true,now(),now())
            RETURNING id INTO v_club_id;
        END IF;

        SELECT array_agg(p.id ORDER BY p.id) INTO v_ids FROM public.players p
        WHERE p.deleted_at IS NULL AND EXISTS (
            SELECT 1 FROM unnest(r.aliases || ARRAY[r.player_name]) n(name)
            WHERE pg_temp.import_name_key(p.name)=pg_temp.import_name_key(n.name)
        );
        IF cardinality(v_ids)>1 THEN
            RAISE EXCEPTION 'Multiple existing profiles match %: %',r.player_name,v_ids;
        END IF;
        v_created := cardinality(v_ids) IS NULL;
        IF NOT v_created THEN
            SELECT * INTO STRICT v_player FROM public.players WHERE id=v_ids[1];
            IF v_player.country_code IS NOT NULL AND v_player.country_code<>r.team_code THEN
                RAISE EXCEPTION 'Existing nationality conflicts for %',r.player_name;
            END IF;
            IF v_player.club_id IS DISTINCT FROM v_club_id THEN
                RAISE EXCEPTION 'Existing club conflicts for %. Existing %, sourced %. Review this registration before importing.',
                    r.player_name,v_player.club_id,v_club_id;
            END IF;
            -- The nationality trigger can clear the national shirt when changing
            -- an Unknown nationality. Set the shirt in a separate statement below.
            UPDATE public.players SET nationality=v_country_name,updated_at=now()
            WHERE id=v_player.id AND country_code IS DISTINCT FROM r.team_code;
        ELSE
            IF EXISTS (
                SELECT 1 FROM public.players p
                WHERE p.deleted_at IS NOT NULL AND EXISTS (
                    SELECT 1 FROM unnest(r.aliases || ARRAY[r.player_name]) n(name)
                    WHERE pg_temp.import_name_key(p.name)=pg_temp.import_name_key(n.name)
                )
            ) THEN
                RAISE EXCEPTION 'A deleted profile matches %. Review before recreating it.',r.player_name;
            END IF;
            INSERT INTO public.players(uuid,name,club_id,jersey_number,position,nationality,
                national_team_id,created_at,updated_at)
            VALUES(gen_random_uuid(),r.player_name,v_club_id,0,r.player_position,v_country_name,
                v_team_id,now(),now()) RETURNING * INTO v_player;
        END IF;

        IF EXISTS (
            SELECT 1 FROM public.players p WHERE p.deleted_at IS NULL
              AND p.national_team_id=v_team_id AND p.national_jersey_number=r.shirt AND p.id<>v_player.id
        ) THEN
            RAISE EXCEPTION 'National shirt % for % is already assigned to another profile',r.shirt,r.team_code;
        END IF;
        UPDATE public.players SET national_team_id=v_team_id,national_jersey_number=r.shirt,updated_at=now()
        WHERE id=v_player.id AND (national_team_id IS DISTINCT FROM v_team_id OR national_jersey_number IS DISTINCT FROM r.shirt);
        INSERT INTO import_results VALUES(r.team_code,r.player_name,v_player.id,v_created,v_club_id);
    END LOOP;

    IF (SELECT count(*) FROM import_results)<>46 OR EXISTS (
        SELECT 1 FROM import_results i
        JOIN import_roster roster ON roster.team_code=i.team_code AND roster.player_name=i.supplied_name
        JOIN public.players p ON p.id=i.player_id
        JOIN import_teams t ON t.team_code=i.team_code
        WHERE p.deleted_at IS NOT NULL OR p.country_code IS DISTINCT FROM roster.team_code
            OR p.national_team_id IS DISTINCT FROM t.team_id
            OR p.national_jersey_number IS DISTINCT FROM roster.shirt OR p.club_id IS DISTINCT FROM i.club_id
    ) THEN
        RAISE EXCEPTION 'Imported player registration validation failed';
    END IF;
END
$import$;

SELECT t.team_code,c.name AS national_team,count(*) AS listed_players,
    count(*) FILTER (WHERE i.was_created) AS added_profiles,
    count(*) FILTER (WHERE NOT i.was_created) AS reused_profiles
FROM import_results i JOIN import_teams t USING(team_code)
JOIN public.clubs c ON c.id=t.team_id
GROUP BY t.team_code,c.name ORDER BY t.team_code;

SELECT i.team_code,p.id,p.name,c.name AS club,c.country AS club_country,
    p.national_jersey_number AS national_shirt,
    CASE WHEN i.was_created THEN 'added' ELSE 'reused' END AS action
FROM import_results i JOIN public.players p ON p.id=i.player_id
JOIN public.clubs c ON c.id=i.club_id
ORDER BY i.team_code,p.national_jersey_number;

COMMIT;
