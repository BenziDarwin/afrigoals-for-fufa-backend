CREATE TABLE public.countries (code varchar(2) PRIMARY KEY, name text NOT NULL UNIQUE, aliases jsonb NOT NULL DEFAULT '[]'::jsonb);
INSERT INTO public.countries (code,name,aliases) VALUES
('AD', 'Andorra', '[]'::jsonb),
('AE', 'United Arab Emirates', '["emirati"]'::jsonb),
('AF', 'Afghanistan', '[]'::jsonb),
('AG', 'Antigua & Barbuda', '[]'::jsonb),
('AI', 'Anguilla', '[]'::jsonb),
('AL', 'Albania', '[]'::jsonb),
('AM', 'Armenia', '[]'::jsonb),
('AO', 'Angola', '["angolan"]'::jsonb),
('AQ', 'Antarctica', '[]'::jsonb),
('AR', 'Argentina', '["argentine","argentinian"]'::jsonb),
('AS', 'American Samoa', '[]'::jsonb),
('AT', 'Austria', '["austrian"]'::jsonb),
('AU', 'Australia', '["australian"]'::jsonb),
('AW', 'Aruba', '[]'::jsonb),
('AX', 'Åland Islands', '[]'::jsonb),
('AZ', 'Azerbaijan', '[]'::jsonb),
('BA', 'Bosnia & Herzegovina', '[]'::jsonb),
('BB', 'Barbados', '[]'::jsonb),
('BD', 'Bangladesh', '[]'::jsonb),
('BE', 'Belgium', '["belgian"]'::jsonb),
('BF', 'Burkina Faso', '["burkinabe"]'::jsonb),
('BG', 'Bulgaria', '[]'::jsonb),
('BH', 'Bahrain', '[]'::jsonb),
('BI', 'Burundi', '["burundian"]'::jsonb),
('BJ', 'Benin', '["beninese"]'::jsonb),
('BL', 'St. Barthélemy', '[]'::jsonb),
('BM', 'Bermuda', '[]'::jsonb),
('BN', 'Brunei', '[]'::jsonb),
('BO', 'Bolivia', '[]'::jsonb),
('BQ', 'Caribbean Netherlands', '[]'::jsonb),
('BR', 'Brazil', '["brazilian"]'::jsonb),
('BS', 'Bahamas', '[]'::jsonb),
('BT', 'Bhutan', '[]'::jsonb),
('BV', 'Bouvet Island', '[]'::jsonb),
('BW', 'Botswana', '["motswana","botswanan"]'::jsonb),
('BY', 'Belarus', '[]'::jsonb),
('BZ', 'Belize', '[]'::jsonb),
('CA', 'Canada', '["canadian"]'::jsonb),
('CC', 'Cocos (Keeling) Islands', '[]'::jsonb),
('CD', 'Congo - Kinshasa', '["congolese","dr congo","democratic republic of congo","drc"]'::jsonb),
('CF', 'Central African Republic', '["central african"]'::jsonb),
('CG', 'Congo - Brazzaville', '["republic of congo"]'::jsonb),
('CH', 'Switzerland', '["swiss"]'::jsonb),
('CI', 'Côte d’Ivoire', '["ivorian","ivory coast"]'::jsonb),
('CK', 'Cook Islands', '[]'::jsonb),
('CL', 'Chile', '["chilean"]'::jsonb),
('CM', 'Cameroon', '["cameroonian"]'::jsonb),
('CN', 'China', '["chinese"]'::jsonb),
('CO', 'Colombia', '["colombian"]'::jsonb),
('CR', 'Costa Rica', '[]'::jsonb),
('CU', 'Cuba', '[]'::jsonb),
('CV', 'Cape Verde', '["cape verdean"]'::jsonb),
('CW', 'Curaçao', '[]'::jsonb),
('CX', 'Christmas Island', '[]'::jsonb),
('CY', 'Cyprus', '[]'::jsonb),
('CZ', 'Czechia', '[]'::jsonb),
('DE', 'Germany', '["german"]'::jsonb),
('DJ', 'Djibouti', '["djiboutian"]'::jsonb),
('DK', 'Denmark', '["danish"]'::jsonb),
('DM', 'Dominica', '[]'::jsonb),
('DO', 'Dominican Republic', '[]'::jsonb),
('DZ', 'Algeria', '["algerian"]'::jsonb),
('EC', 'Ecuador', '[]'::jsonb),
('EE', 'Estonia', '[]'::jsonb),
('EG', 'Egypt', '["egyptian"]'::jsonb),
('EH', 'Western Sahara', '[]'::jsonb),
('ER', 'Eritrea', '["eritrean"]'::jsonb),
('ES', 'Spain', '["spanish"]'::jsonb),
('ET', 'Ethiopia', '["ethiopian"]'::jsonb),
('FI', 'Finland', '["finnish"]'::jsonb),
('FJ', 'Fiji', '[]'::jsonb),
('FK', 'Falkland Islands', '[]'::jsonb),
('FM', 'Micronesia', '[]'::jsonb),
('FO', 'Faroe Islands', '[]'::jsonb),
('FR', 'France', '["french"]'::jsonb),
('GA', 'Gabon', '["gabonese"]'::jsonb),
('GB', 'United Kingdom', '["british","united kingdom","uk"]'::jsonb),
('GD', 'Grenada', '[]'::jsonb),
('GE', 'Georgia', '[]'::jsonb),
('GF', 'French Guiana', '[]'::jsonb),
('GG', 'Guernsey', '[]'::jsonb),
('GH', 'Ghana', '["ghanaian"]'::jsonb),
('GI', 'Gibraltar', '[]'::jsonb),
('GL', 'Greenland', '[]'::jsonb),
('GM', 'Gambia', '["gambian"]'::jsonb),
('GN', 'Guinea', '["guinean"]'::jsonb),
('GP', 'Guadeloupe', '[]'::jsonb),
('GQ', 'Equatorial Guinea', '["equatoguinean"]'::jsonb),
('GR', 'Greece', '[]'::jsonb),
('GS', 'South Georgia & South Sandwich Islands', '[]'::jsonb),
('GT', 'Guatemala', '[]'::jsonb),
('GU', 'Guam', '[]'::jsonb),
('GW', 'Guinea-Bissau', '["bissau-guinean"]'::jsonb),
('GY', 'Guyana', '[]'::jsonb),
('HK', 'Hong Kong SAR China', '[]'::jsonb),
('HM', 'Heard & McDonald Islands', '[]'::jsonb),
('HN', 'Honduras', '[]'::jsonb),
('HR', 'Croatia', '[]'::jsonb),
('HT', 'Haiti', '[]'::jsonb),
('HU', 'Hungary', '[]'::jsonb),
('ID', 'Indonesia', '[]'::jsonb),
('IE', 'Ireland', '["irish"]'::jsonb),
('IL', 'Israel', '["israeli"]'::jsonb),
('IM', 'Isle of Man', '[]'::jsonb),
('IN', 'India', '["indian"]'::jsonb),
('IO', 'British Indian Ocean Territory', '[]'::jsonb),
('IQ', 'Iraq', '[]'::jsonb),
('IR', 'Iran', '[]'::jsonb),
('IS', 'Iceland', '[]'::jsonb),
('IT', 'Italy', '["italian"]'::jsonb),
('JE', 'Jersey', '[]'::jsonb),
('JM', 'Jamaica', '[]'::jsonb),
('JO', 'Jordan', '[]'::jsonb),
('JP', 'Japan', '["japanese"]'::jsonb),
('KE', 'Kenya', '["kenyan"]'::jsonb),
('KG', 'Kyrgyzstan', '[]'::jsonb),
('KH', 'Cambodia', '[]'::jsonb),
('KI', 'Kiribati', '[]'::jsonb),
('KM', 'Comoros', '["comorian"]'::jsonb),
('KN', 'St. Kitts & Nevis', '[]'::jsonb),
('KP', 'North Korea', '[]'::jsonb),
('KR', 'South Korea', '["south korean"]'::jsonb),
('KW', 'Kuwait', '[]'::jsonb),
('KY', 'Cayman Islands', '[]'::jsonb),
('KZ', 'Kazakhstan', '[]'::jsonb),
('LA', 'Laos', '[]'::jsonb),
('LB', 'Lebanon', '[]'::jsonb),
('LC', 'St. Lucia', '[]'::jsonb),
('LI', 'Liechtenstein', '[]'::jsonb),
('LK', 'Sri Lanka', '[]'::jsonb),
('LR', 'Liberia', '["liberian"]'::jsonb),
('LS', 'Lesotho', '["basotho","mosotho"]'::jsonb),
('LT', 'Lithuania', '[]'::jsonb),
('LU', 'Luxembourg', '[]'::jsonb),
('LV', 'Latvia', '[]'::jsonb),
('LY', 'Libya', '["libyan"]'::jsonb),
('MA', 'Morocco', '["moroccan"]'::jsonb),
('MC', 'Monaco', '[]'::jsonb),
('MD', 'Moldova', '[]'::jsonb),
('ME', 'Montenegro', '[]'::jsonb),
('MF', 'St. Martin', '[]'::jsonb),
('MG', 'Madagascar', '["malagasy"]'::jsonb),
('MH', 'Marshall Islands', '[]'::jsonb),
('MK', 'North Macedonia', '[]'::jsonb),
('ML', 'Mali', '["malian"]'::jsonb),
('MM', 'Myanmar (Burma)', '[]'::jsonb),
('MN', 'Mongolia', '[]'::jsonb),
('MO', 'Macao SAR China', '[]'::jsonb),
('MP', 'Northern Mariana Islands', '[]'::jsonb),
('MQ', 'Martinique', '[]'::jsonb),
('MR', 'Mauritania', '[]'::jsonb),
('MS', 'Montserrat', '[]'::jsonb),
('MT', 'Malta', '[]'::jsonb),
('MU', 'Mauritius', '["mauritian"]'::jsonb),
('MV', 'Maldives', '[]'::jsonb),
('MW', 'Malawi', '["malawian"]'::jsonb),
('MX', 'Mexico', '["mexican"]'::jsonb),
('MY', 'Malaysia', '[]'::jsonb),
('MZ', 'Mozambique', '["mozambican"]'::jsonb),
('NA', 'Namibia', '["namibian"]'::jsonb),
('NC', 'New Caledonia', '[]'::jsonb),
('NE', 'Niger', '["nigerien"]'::jsonb),
('NF', 'Norfolk Island', '[]'::jsonb),
('NG', 'Nigeria', '["nigerian"]'::jsonb),
('NI', 'Nicaragua', '[]'::jsonb),
('NL', 'Netherlands', '["dutch"]'::jsonb),
('NO', 'Norway', '["norwegian"]'::jsonb),
('NP', 'Nepal', '[]'::jsonb),
('NR', 'Nauru', '[]'::jsonb),
('NU', 'Niue', '[]'::jsonb),
('NZ', 'New Zealand', '["new zealander"]'::jsonb),
('OM', 'Oman', '[]'::jsonb),
('PA', 'Panama', '[]'::jsonb),
('PE', 'Peru', '[]'::jsonb),
('PF', 'French Polynesia', '[]'::jsonb),
('PG', 'Papua New Guinea', '[]'::jsonb),
('PH', 'Philippines', '[]'::jsonb),
('PK', 'Pakistan', '["pakistani"]'::jsonb),
('PL', 'Poland', '["polish"]'::jsonb),
('PM', 'St. Pierre & Miquelon', '[]'::jsonb),
('PN', 'Pitcairn Islands', '[]'::jsonb),
('PR', 'Puerto Rico', '[]'::jsonb),
('PS', 'Palestinian Territories', '["palestinian"]'::jsonb),
('PT', 'Portugal', '["portuguese"]'::jsonb),
('PW', 'Palau', '[]'::jsonb),
('PY', 'Paraguay', '[]'::jsonb),
('QA', 'Qatar', '["qatari"]'::jsonb),
('RE', 'Réunion', '[]'::jsonb),
('RO', 'Romania', '[]'::jsonb),
('RS', 'Serbia', '[]'::jsonb),
('RU', 'Russia', '["russian"]'::jsonb),
('RW', 'Rwanda', '["rwandan"]'::jsonb),
('SA', 'Saudi Arabia', '["saudi arabian"]'::jsonb),
('SB', 'Solomon Islands', '[]'::jsonb),
('SC', 'Seychelles', '["seychellois"]'::jsonb),
('SD', 'Sudan', '["sudanese"]'::jsonb),
('SE', 'Sweden', '["swedish"]'::jsonb),
('SG', 'Singapore', '[]'::jsonb),
('SH', 'St. Helena', '[]'::jsonb),
('SI', 'Slovenia', '[]'::jsonb),
('SJ', 'Svalbard & Jan Mayen', '[]'::jsonb),
('SK', 'Slovakia', '[]'::jsonb),
('SL', 'Sierra Leone', '["sierra leonean"]'::jsonb),
('SM', 'San Marino', '[]'::jsonb),
('SN', 'Senegal', '["senegalese"]'::jsonb),
('SO', 'Somalia', '["somali"]'::jsonb),
('SR', 'Suriname', '[]'::jsonb),
('SS', 'South Sudan', '["south sudanese"]'::jsonb),
('ST', 'São Tomé & Príncipe', '["sao tomean"]'::jsonb),
('SV', 'El Salvador', '[]'::jsonb),
('SX', 'Sint Maarten', '[]'::jsonb),
('SY', 'Syria', '[]'::jsonb),
('SZ', 'Eswatini', '["swazi","eswatini"]'::jsonb),
('TC', 'Turks & Caicos Islands', '[]'::jsonb),
('TD', 'Chad', '["chadian"]'::jsonb),
('TF', 'French Southern Territories', '[]'::jsonb),
('TG', 'Togo', '["togolese"]'::jsonb),
('TH', 'Thailand', '[]'::jsonb),
('TJ', 'Tajikistan', '[]'::jsonb),
('TK', 'Tokelau', '[]'::jsonb),
('TL', 'Timor-Leste', '[]'::jsonb),
('TM', 'Turkmenistan', '[]'::jsonb),
('TN', 'Tunisia', '["tunisian"]'::jsonb),
('TO', 'Tonga', '[]'::jsonb),
('TR', 'Türkiye', '["turkish","turkey"]'::jsonb),
('TT', 'Trinidad & Tobago', '[]'::jsonb),
('TV', 'Tuvalu', '[]'::jsonb),
('TW', 'Taiwan', '[]'::jsonb),
('TZ', 'Tanzania', '["tanzanian"]'::jsonb),
('UA', 'Ukraine', '["ukrainian"]'::jsonb),
('UG', 'Uganda', '["ugandan"]'::jsonb),
('UM', 'U.S. Outlying Islands', '[]'::jsonb),
('US', 'United States', '["american","united states","usa"]'::jsonb),
('UY', 'Uruguay', '["uruguayan"]'::jsonb),
('UZ', 'Uzbekistan', '[]'::jsonb),
('VA', 'Vatican City', '[]'::jsonb),
('VC', 'St. Vincent & Grenadines', '[]'::jsonb),
('VE', 'Venezuela', '[]'::jsonb),
('VG', 'British Virgin Islands', '[]'::jsonb),
('VI', 'U.S. Virgin Islands', '[]'::jsonb),
('VN', 'Vietnam', '[]'::jsonb),
('VU', 'Vanuatu', '[]'::jsonb),
('WF', 'Wallis & Futuna', '[]'::jsonb),
('WS', 'Samoa', '[]'::jsonb),
('YE', 'Yemen', '[]'::jsonb),
('YT', 'Mayotte', '[]'::jsonb),
('ZA', 'South Africa', '["south african"]'::jsonb),
('ZM', 'Zambia', '["zambian"]'::jsonb),
('ZW', 'Zimbabwe', '["zimbabwean"]'::jsonb);

ALTER TABLE public.leagues ADD COLUMN country_code varchar(2) REFERENCES public.countries(code);
ALTER TABLE public.clubs ADD COLUMN country_code varchar(2) REFERENCES public.countries(code);
ALTER TABLE public.clubs ADD COLUMN is_external boolean NOT NULL DEFAULT false;
ALTER TABLE public.players ADD COLUMN country_code varchar(2) REFERENCES public.countries(code);
CREATE INDEX idx_players_country ON public.players(country_code) WHERE deleted_at IS NULL;

CREATE FUNCTION public.afrigoals_country_code(value text) RETURNS varchar(2)
LANGUAGE sql STABLE AS $$
 SELECT code FROM public.countries
 WHERE lower(btrim(value)) IN (lower(code),lower(name)) OR aliases ? lower(btrim(value))
 ORDER BY code LIMIT 1
$$;

UPDATE public.leagues SET country_code=public.afrigoals_country_code(country);
UPDATE public.clubs SET country_code=public.afrigoals_country_code(country);
UPDATE public.players SET country_code=public.afrigoals_country_code(nationality);
-- Existing club records may predate the country field. Infer only a single
-- unambiguous country from the club's registered leagues.
UPDATE public.clubs c SET country_code=q.code, country=co.name
FROM (SELECT cl.club_id,min(l.country_code) AS code FROM public.club_leagues cl
 JOIN public.leagues l ON l.id=cl.league_id AND l.deleted_at IS NULL
 WHERE l.country_code IS NOT NULL GROUP BY cl.club_id HAVING count(DISTINCT l.country_code)=1) q
JOIN public.countries co ON co.code=q.code
WHERE c.id=q.club_id AND c.country_code IS NULL AND btrim(COALESCE(c.country,''))='';

CREATE FUNCTION public.afrigoals_normalize_team_country() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.country_code := public.afrigoals_country_code(NEW.country);
 IF btrim(COALESCE(NEW.country,''))<>'' AND NEW.country_code IS NULL THEN
  RAISE EXCEPTION 'Select a country from the country directory' USING ERRCODE='23514';
 END IF;
 IF NEW.country_code IS NOT NULL THEN
  SELECT name INTO NEW.country FROM public.countries WHERE code=NEW.country_code;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER normalize_club_country BEFORE INSERT OR UPDATE OF country ON public.clubs
 FOR EACH ROW EXECUTE FUNCTION public.afrigoals_normalize_team_country();
CREATE TRIGGER normalize_league_country BEFORE INSERT OR UPDATE OF country ON public.leagues
 FOR EACH ROW EXECUTE FUNCTION public.afrigoals_normalize_team_country();

CREATE FUNCTION public.afrigoals_player_nationality() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE selected_country varchar(2); selected_team bigint;
BEGIN
 NEW.country_code := public.afrigoals_country_code(NEW.nationality);
 IF btrim(COALESCE(NEW.nationality,''))<>'' AND NEW.country_code IS NULL THEN
  RAISE EXCEPTION 'Select the player country from the country directory' USING ERRCODE='23514';
 END IF;
 IF TG_OP='UPDATE' AND OLD.country_code IS DISTINCT FROM NEW.country_code THEN
  NEW.national_team_id := NULL;
  NEW.national_jersey_number := NULL;
 END IF;
 IF NEW.national_team_id IS NOT NULL THEN
  SELECT country_code INTO selected_country FROM public.clubs
  WHERE id=NEW.national_team_id AND team_type='national' AND deleted_at IS NULL;
  IF selected_country IS NULL OR selected_country IS DISTINCT FROM NEW.country_code THEN
   RAISE EXCEPTION 'Player nationality must match the national team country' USING ERRCODE='23514';
  END IF;
 ELSE
  SELECT id INTO selected_team FROM public.clubs WHERE team_type='national'
   AND country_code=NEW.country_code AND deleted_at IS NULL ORDER BY id LIMIT 1;
  NEW.national_team_id := selected_team;
  NEW.national_jersey_number := NULL;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER player_nationality_registration BEFORE INSERT OR UPDATE OF nationality,national_team_id
 ON public.players FOR EACH ROW EXECUTE FUNCTION public.afrigoals_player_nationality();

CREATE FUNCTION public.afrigoals_populate_national_team() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' THEN
  UPDATE public.players SET national_team_id=NULL,national_jersey_number=NULL
  WHERE national_team_id=NEW.id AND (NEW.team_type<>'national' OR NEW.deleted_at IS NOT NULL OR country_code IS DISTINCT FROM NEW.country_code);
 END IF;
 IF NEW.team_type='national' AND NEW.country_code IS NOT NULL AND NEW.deleted_at IS NULL THEN
  UPDATE public.players SET national_team_id=NEW.id,national_jersey_number=NULL
  WHERE country_code=NEW.country_code AND national_team_id IS NULL AND deleted_at IS NULL;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER populate_national_team AFTER INSERT OR UPDATE OF team_type,country,deleted_at ON public.clubs
 FOR EACH ROW EXECUTE FUNCTION public.afrigoals_populate_national_team();
UPDATE public.players p SET national_team_id=(SELECT c.id FROM public.clubs c
 WHERE c.team_type='national' AND c.country_code=p.country_code AND c.deleted_at IS NULL ORDER BY c.id LIMIT 1)
WHERE p.national_team_id IS NULL AND p.country_code IS NOT NULL AND p.deleted_at IS NULL;

CREATE TABLE public.player_club_histories (
 id bigserial PRIMARY KEY,
 player_id bigint NOT NULL REFERENCES public.players(id),
 from_club_id bigint REFERENCES public.clubs(id),
 to_club_id bigint REFERENCES public.clubs(id),
 previous_jersey integer NOT NULL DEFAULT 0,
 new_jersey integer NOT NULL DEFAULT 0,
 transferred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_player_club_history ON public.player_club_histories(player_id,transferred_at);
CREATE FUNCTION public.afrigoals_record_club_transfer() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.club_id IS DISTINCT FROM NEW.club_id THEN
  INSERT INTO public.player_club_histories(player_id,from_club_id,to_club_id,previous_jersey,new_jersey)
   VALUES(NEW.id,OLD.club_id,NEW.club_id,COALESCE(OLD.jersey_number,0),COALESCE(NEW.jersey_number,0));
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER record_player_club_transfer AFTER UPDATE OF club_id ON public.players
 FOR EACH ROW EXECUTE FUNCTION public.afrigoals_record_club_transfer();

CREATE FUNCTION public.afrigoals_register_club_league() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE public.clubs SET is_external=false WHERE id=NEW.club_id;
 RETURN NEW;
END $$;
CREATE TRIGGER register_club_league AFTER INSERT ON public.club_leagues
 FOR EACH ROW EXECUTE FUNCTION public.afrigoals_register_club_league();
