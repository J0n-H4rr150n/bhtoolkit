CREATE TABLE IF NOT EXISTS synack_missions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_task_id TEXT NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    campaign_name TEXT,
    organization_uid TEXT,
    listing_uid TEXT,
    listing_codename TEXT,
    campaign_uid TEXT,
    payout_amount REAL,
    payout_currency TEXT,
    status TEXT NOT NULL DEFAULT 'UNKNOWN', -- e.g., CLAIM_INITIATED, CLAIMED, FAILED_TO_CLAIM, SUBMITTED, PAID
    claimed_by_toolkit_at TIMESTAMP, -- Timestamp when the toolkit attempted/confirmed claim
    synack_api_claimed_at TIMESTAMP, -- Timestamp from Synack if available, or our successful claim time
    synack_api_expires_at TIMESTAMP, -- Expiration timestamp from Synack
    raw_mission_details_json TEXT,
    notes TEXT, -- For any internal notes about this mission
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_synack_missions_status ON synack_missions(status);
CREATE INDEX IF NOT EXISTS idx_synack_missions_claimed_by_toolkit_at ON synack_missions(claimed_by_toolkit_at);