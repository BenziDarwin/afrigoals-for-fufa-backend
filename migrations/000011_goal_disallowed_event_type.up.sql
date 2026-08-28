INSERT INTO public.event_types
    (name, value, category, shortcut, priority, requires_player, requires_secondary_player)
VALUES
    ('Goal Disallowed', 'goal_disallowed', 'attack', null, 'critical', true, false)
ON CONFLICT (value) DO UPDATE SET
    name = EXCLUDED.name,
    category = EXCLUDED.category,
    shortcut = EXCLUDED.shortcut,
    priority = EXCLUDED.priority,
    requires_player = EXCLUDED.requires_player,
    requires_secondary_player = EXCLUDED.requires_secondary_player,
    updated_at = now();
