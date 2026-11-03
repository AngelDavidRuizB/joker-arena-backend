-- ==============================================================================
-- liveops-rules-ms · esquema "liveops" (dueño: svc_liveops)
-- Versiones append-only del ruleset. La v1 reproduce EXACTAMENTE el Prototipo 1.
-- ==============================================================================
SET ROLE svc_liveops;

CREATE TABLE IF NOT EXISTS liveops.liveops_rulesets (
    version     INTEGER     PRIMARY KEY CHECK (version >= 1),
    payload     JSONB       NOT NULL,
    updated_by  VARCHAR(64) NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO liveops.liveops_rulesets (version, payload, updated_by) VALUES (1, '{
  "jokers": {
    "banner":  {"enabled": true, "chips_per_discard": 30},
    "classic": {"enabled": true, "mult_bonus": 4},
    "duo":     {"enabled": true, "mult_factor": 2}
  },
  "happy_hour": {"enabled": false, "global_mult": 1}
}', 'seed') ON CONFLICT (version) DO NOTHING;

RESET ROLE;
