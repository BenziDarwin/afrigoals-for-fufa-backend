CREATE TABLE IF NOT EXISTS public.event_types (
    id bigserial PRIMARY KEY,
    name character varying(120) NOT NULL,
    value character varying(80) NOT NULL,
    category character varying(80) NOT NULL,
    shortcut character varying(8),
    priority character varying(20) NOT NULL DEFAULT 'low',
    requires_player boolean NOT NULL DEFAULT false,
    requires_secondary_player boolean NOT NULL DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_event_types_name ON public.event_types USING btree (name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_types_value ON public.event_types USING btree (value);
CREATE INDEX IF NOT EXISTS idx_event_types_category ON public.event_types USING btree (category);
CREATE INDEX IF NOT EXISTS idx_event_types_priority ON public.event_types USING btree (priority);

ALTER TABLE public.analysis_events
    ADD COLUMN IF NOT EXISTS event_type_id bigint,
    ADD COLUMN IF NOT EXISTS team_id bigint,
    ADD COLUMN IF NOT EXISTS secondary_player_id bigint,
    ADD COLUMN IF NOT EXISTS pitch_zone character varying(80),
    ADD COLUMN IF NOT EXISTS outcome character varying(80),
    ADD COLUMN IF NOT EXISTS confidence_score double precision;

CREATE INDEX IF NOT EXISTS idx_analysis_events_event_type_id ON public.analysis_events USING btree (event_type_id);
CREATE INDEX IF NOT EXISTS idx_analysis_events_team_id ON public.analysis_events USING btree (team_id);
CREATE INDEX IF NOT EXISTS idx_analysis_events_secondary_player_id ON public.analysis_events USING btree (secondary_player_id);

INSERT INTO public.event_types
    (name, value, category, shortcut, priority, requires_player, requires_secondary_player)
VALUES
    ('Goal', 'goal', 'attack', 'g', 'critical', true, true),
    ('Shot', 'shot', 'attack', 's', 'critical', true, false),
    ('Shot On Target', 'shot_on_target', 'attack', 'o', 'critical', true, true),
    ('Big Chance', 'big_chance', 'attack', 'm', 'critical', true, false),
    ('Assist', 'assist', 'attack', 'a', 'critical', true, true),
    ('Key Pass', 'key_pass', 'attack', 'k', 'high', true, true),
    ('Through Ball', 'through_ball', 'attack', null, 'high', true, true),
    ('Cross', 'cross', 'attack', 'x', 'high', true, true),
    ('Cutback', 'cutback', 'attack', null, 'high', true, true),
    ('Dribble', 'dribble', 'attack', 'd', 'high', true, true),
    ('1v1 Attack', 'one_v_one_attack', 'attack', null, 'high', true, true),

    ('Pass Completed', 'pass_completed', 'passing', 'p', 'medium', true, true),
    ('Pass Failed', 'pass_failed', 'passing', null, 'medium', true, true),
    ('Progressive Pass', 'progressive_pass', 'passing', null, 'medium', true, true),
    ('Long Ball', 'long_ball', 'passing', 'l', 'medium', true, true),
    ('Switch Of Play', 'switch_of_play', 'passing', null, 'medium', true, true),
    ('Pass Into Final Third', 'pass_into_final_third', 'passing', null, 'medium', true, true),
    ('Pass Into Penalty Area', 'pass_into_penalty_area', 'passing', null, 'medium', true, true),
    ('One Touch Pass', 'one_touch_pass', 'passing', null, 'medium', true, true),
    ('Combination Play', 'combination_play', 'passing', null, 'medium', true, true),

    ('Tackle', 'tackle', 'defence', 't', 'high', true, true),
    ('Interception', 'interception', 'defence', 'i', 'high', true, false),
    ('Clearance', 'clearance', 'defence', 'c', 'medium', true, false),
    ('Block', 'block', 'defence', 'b', 'medium', true, false),
    ('Aerial Duel', 'aerial_duel', 'defence', null, 'medium', true, true),
    ('Ground Duel', 'ground_duel', 'defence', null, 'medium', true, true),
    ('Recovery', 'recovery', 'defence', 'r', 'medium', true, false),
    ('Marking Error', 'marking_error', 'defence', null, 'critical', true, false),
    ('Positional Error', 'positional_error', 'defence', null, 'critical', true, false),
    ('Last Man Defence', 'last_man_defence', 'defence', null, 'high', true, true),
    ('Goal Line Clearance', 'goal_line_clearance', 'defence', null, 'critical', true, false),

    ('Possession Won', 'possession_won', 'possession', null, 'medium', true, false),
    ('Possession Lost', 'possession_lost', 'possession', 'w', 'critical', true, false),
    ('Dangerous Turnover', 'dangerous_turnover', 'possession', null, 'critical', true, false),
    ('Bad Touch', 'bad_touch', 'possession', null, 'medium', true, false),
    ('Miscontrol', 'miscontrol', 'possession', null, 'medium', true, false),
    ('Ball Carry', 'ball_carry', 'possession', null, 'medium', true, false),
    ('Carry Into Final Third', 'carry_into_final_third', 'possession', null, 'medium', true, false),
    ('Pressure Received', 'pressure_received', 'possession', null, 'medium', true, true),
    ('Pressure Escaped', 'pressure_escaped', 'possession', null, 'medium', true, true),

    ('High Press', 'high_press', 'transition', 'h', 'high', false, false),
    ('Mid Block', 'mid_block', 'transition', null, 'medium', false, false),
    ('Low Block', 'low_block', 'transition', null, 'medium', false, false),
    ('Pressing Action', 'pressing_action', 'transition', null, 'high', true, false),
    ('Press Success', 'press_success', 'transition', null, 'high', true, false),
    ('Counter Press', 'counter_press', 'transition', null, 'high', false, false),
    ('Counter Attack', 'counter_attack', 'transition', 'n', 'critical', false, false),
    ('Fast Break', 'fast_break', 'transition', null, 'critical', false, false),
    ('Attacking Transition', 'attacking_transition', 'transition', null, 'high', false, false),
    ('Defensive Transition', 'defensive_transition', 'transition', null, 'high', false, false),
    ('Recovery After Loss', 'recovery_after_loss', 'transition', null, 'high', true, false),
    ('Time To Shoot After Recovery', 'time_to_shoot_after_recovery', 'transition', null, 'high', false, false),

    ('Save', 'save', 'goalkeeper', null, 'critical', true, false),
    ('One vs One Save', 'one_vs_one_save', 'goalkeeper', null, 'critical', true, true),
    ('Catch Cross', 'catch_cross', 'goalkeeper', null, 'medium', true, false),
    ('Punch', 'punch', 'goalkeeper', null, 'medium', true, false),
    ('Claim High Ball', 'claim_high_ball', 'goalkeeper', null, 'medium', true, false),
    ('Distribution Short', 'distribution_short', 'goalkeeper', null, 'medium', true, true),
    ('Distribution Long', 'distribution_long', 'goalkeeper', null, 'medium', true, true),
    ('Throw Distribution', 'throw_distribution', 'goalkeeper', null, 'medium', true, true),
    ('Sweeper Action', 'sweeper_action', 'goalkeeper', null, 'high', true, false),
    ('Goalkeeper Error', 'goalkeeper_error', 'goalkeeper', null, 'critical', true, false),

    ('Corner Kick', 'corner_kick', 'set_piece', 'q', 'medium', true, true),
    ('Free Kick', 'free_kick', 'set_piece', null, 'medium', true, false),
    ('Penalty', 'penalty', 'set_piece', null, 'critical', true, true),
    ('Throw In', 'throw_in', 'set_piece', null, 'low', true, true),
    ('Second Ball Recovery', 'second_ball_recovery', 'set_piece', null, 'medium', true, false),
    ('Set Piece Chance Created', 'set_piece_chance_created', 'set_piece', null, 'high', true, true),
    ('Set Piece Goal', 'set_piece_goal', 'set_piece', null, 'critical', true, true),

    ('Foul Committed', 'foul_committed', 'discipline', 'f', 'low', true, true),
    ('Foul Suffered', 'foul_suffered', 'discipline', null, 'low', true, true),
    ('Yellow Card', 'yellow_card', 'discipline', 'y', 'low', true, false),
    ('Second Yellow', 'second_yellow', 'discipline', null, 'low', true, false),
    ('Red Card', 'red_card', 'discipline', null, 'low', true, false),
    ('Advantage Play', 'advantage_play', 'discipline', null, 'low', false, false),
    ('Time Wasting', 'time_wasting', 'discipline', null, 'low', true, false),
    ('Dissent', 'dissent', 'discipline', null, 'low', true, false),

    ('Attacking Run', 'attacking_run', 'movement', null, 'medium', true, false),
    ('Overlap Run', 'overlap_run', 'movement', null, 'medium', true, false),
    ('Underlap Run', 'underlap_run', 'movement', null, 'medium', true, false),
    ('Run Behind Defence', 'run_behind_defence', 'movement', null, 'medium', true, false),
    ('Support Run', 'support_run', 'movement', null, 'medium', true, false),
    ('Recovery Run', 'recovery_run', 'movement', null, 'medium', true, false),
    ('Position Change', 'position_change', 'movement', null, 'medium', true, false),
    ('Line Breaking Movement', 'line_breaking_movement', 'movement', null, 'medium', true, false),
    ('Offside', 'offside', 'movement', null, 'low', true, false),

    ('Formation Change', 'formation_change', 'tactical', null, 'medium', false, false),
    ('Pressing Style Change', 'pressing_style_change', 'tactical', null, 'medium', false, false),
    ('Defensive Shape', 'defensive_shape', 'tactical', null, 'medium', false, false),
    ('Attacking Pattern', 'attacking_pattern', 'tactical', null, 'medium', false, false),
    ('Build Up Phase', 'build_up_phase', 'tactical', null, 'medium', false, false),
    ('Possession Phase', 'possession_phase', 'tactical', null, 'medium', false, false),
    ('Transition Phase', 'transition_phase', 'tactical', null, 'medium', false, false),
    ('Player Rotation', 'player_rotation', 'tactical', null, 'medium', true, false),
    ('Numerical Advantage Created', 'numerical_advantage_created', 'tactical', null, 'high', false, false),
    ('Numerical Disadvantage Created', 'numerical_disadvantage_created', 'tactical', null, 'high', false, false),

    ('Defensive Error', 'defensive_error', 'error', 'e', 'critical', true, false),
    ('Attacking Error', 'attacking_error', 'error', null, 'critical', true, false),
    ('Failed Clearance', 'failed_clearance', 'error', null, 'critical', true, false),
    ('Misplaced Pass', 'misplaced_pass', 'error', null, 'critical', true, true),
    ('Loss In Dangerous Area', 'loss_in_dangerous_area', 'error', null, 'critical', true, false),
    ('Miscommunication', 'miscommunication', 'error', null, 'critical', true, true),
    ('Wrong Decision', 'wrong_decision', 'error', null, 'critical', true, false),
    ('Failed Press', 'failed_press', 'error', null, 'critical', true, false),
    ('Failed Cover', 'failed_cover', 'error', null, 'critical', true, false)
ON CONFLICT (value) DO UPDATE SET
    name = EXCLUDED.name,
    category = EXCLUDED.category,
    shortcut = EXCLUDED.shortcut,
    priority = EXCLUDED.priority,
    requires_player = EXCLUDED.requires_player,
    requires_secondary_player = EXCLUDED.requires_secondary_player,
    updated_at = now();
