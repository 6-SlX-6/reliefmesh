-- ReliefMesh initial schema (v0.1.0)
--
-- Design notes:
--  * Every table that holds operational data carries organization_id so a
--    future multi-organization release does not need a data migration.
--  * Protected values (exact locations, contact details) are stored only as
--    application-level AES-256-GCM ciphertext ("*_sealed" columns). The
--    database never sees the plaintext.
--  * audit_events is append-only. Triggers reject UPDATE, DELETE and
--    TRUNCATE, and every row carries a SHA-256 hash chain so tampering at the
--    storage level is detectable (see internal/audit).
--  * Quantity invariants (no over-allocation, remaining >= 0) are enforced by
--    CHECK constraints in addition to row locks in the service layer.

CREATE TABLE organizations (
    id          uuid PRIMARY KEY,
    name        text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id                    uuid PRIMARY KEY,
    organization_id       uuid NOT NULL REFERENCES organizations(id),
    username              text NOT NULL CHECK (char_length(username) BETWEEN 3 AND 64),
    display_name          text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80),
    roles                 text[] NOT NULL,
    password_hash         text NOT NULL,
    is_active             boolean NOT NULL DEFAULT true,
    must_change_password  boolean NOT NULL DEFAULT false,
    failed_login_count    integer NOT NULL DEFAULT 0,
    locked_until          timestamptz,
    availability          text NOT NULL DEFAULT 'unavailable'
                          CHECK (availability IN ('available', 'limited', 'unavailable')),
    availability_note     text NOT NULL DEFAULT '' CHECK (char_length(availability_note) <= 200),
    last_login_at         timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_roles_valid CHECK (
        cardinality(roles) >= 1
        AND roles <@ ARRAY['requester', 'volunteer', 'coordinator', 'organization_manager', 'admin']::text[]
    )
);
CREATE UNIQUE INDEX users_username_key ON users (lower(username));
CREATE INDEX users_org_idx ON users (organization_id);

CREATE TABLE sessions (
    id            uuid PRIMARY KEY,
    token_hash    bytea NOT NULL UNIQUE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE instance_settings (
    id                          smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    instance_name               text NOT NULL DEFAULT 'ReliefMesh' CHECK (char_length(instance_name) BETWEEN 1 AND 80),
    emergency_notice            text NOT NULL CHECK (char_length(emergency_notice) BETWEEN 10 AND 1000),
    critical_urgency_notice     text NOT NULL CHECK (char_length(critical_urgency_notice) BETWEEN 10 AND 1500),
    medicine_privacy_notice     text NOT NULL CHECK (char_length(medicine_privacy_notice) BETWEEN 10 AND 1500),
    exercise_mode               boolean NOT NULL DEFAULT true,
    exercise_label              text NOT NULL DEFAULT 'EXERCISE - not a real emergency' CHECK (char_length(exercise_label) BETWEEN 1 AND 120),
    approx_location_decimals    smallint NOT NULL DEFAULT 2 CHECK (approx_location_decimals BETWEEN 0 AND 3),
    retention_days_closed       integer NOT NULL DEFAULT 90 CHECK (retention_days_closed BETWEEN 1 AND 3650),
    default_request_expiry_hours integer NOT NULL DEFAULT 72 CHECK (default_request_expiry_hours BETWEEN 1 AND 8760),
    allow_medicine_free_text    boolean NOT NULL DEFAULT false,
    volunteer_access_requires_grant boolean NOT NULL DEFAULT true,
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    updated_by_user_id          uuid REFERENCES users(id)
);

INSERT INTO instance_settings (id, emergency_notice, critical_urgency_notice, medicine_privacy_notice)
VALUES (
    1,
    'Emergency notice: If there is immediate danger, contact your local emergency services. In the EU, call 112 where available. ReliefMesh is not an emergency dispatch service.',
    'You selected CRITICAL urgency. If anyone is in immediate danger to life or health, or there is a fire, crime or other emergency, stop and contact your local emergency services now (in the EU, call 112 where available). Do not wait for a reply in ReliefMesh. Submitting here does NOT notify emergency services or authorities.',
    'Medicine pickup requests are handled as logistics only. Do not enter diagnoses, prescription details, medication names or medical records. A coordinator will contact you through your chosen contact method to arrange the pickup. ReliefMesh does not give medical advice.'
);

CREATE TABLE categories (
    code          text PRIMARY KEY CHECK (code ~ '^[a-z_]{2,40}$'),
    label         text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 60),
    description   text NOT NULL DEFAULT '' CHECK (char_length(description) <= 300),
    is_sensitive  boolean NOT NULL DEFAULT false,
    enabled       boolean NOT NULL DEFAULT true,
    sort_order    integer NOT NULL DEFAULT 0
);

INSERT INTO categories (code, label, description, is_sensitive, sort_order) VALUES
    ('drinking_water',        'Drinking water',        'Bottled or treated drinking water.', false, 10),
    ('food',                  'Food',                  'Meals, groceries or non-perishable food.', false, 20),
    ('shelter',               'Shelter',               'Shelter places or temporary accommodation capacity.', false, 30),
    ('blankets',              'Blankets',              'Blankets, sleeping bags, warm clothing.', false, 40),
    ('hygiene',               'Hygiene products',      'Soap, sanitary products, cleaning supplies.', false, 50),
    ('baby_supplies',         'Baby supplies',         'Diapers, baby food, baby care items.', false, 60),
    ('power_charging',        'Power / charging',      'Charging for phones and essential devices.', false, 70),
    ('transport',             'Transport',             'Non-emergency transport of people or goods.', false, 80),
    ('volunteer_support',     'Volunteer support',     'Hands-on help from volunteers.', false, 90),
    ('translation',           'Translation',           'Language and interpretation support.', false, 100),
    ('accessibility_support', 'Accessibility support', 'Support for people with disabilities or reduced mobility.', false, 110),
    ('information',           'Information',           'Local, non-emergency information requests.', false, 120),
    ('pet_animal_support',    'Pet / animal support',  'Food, shelter or care for pets and animals.', false, 130),
    ('medicine_pickup',       'Medicine pickup',       'Logistics only: picking up already-arranged items. No medical details.', true, 140),
    ('other',                 'Other',                 'Anything not covered by another category.', false, 150);

CREATE TABLE reference_counters (
    prefix  text NOT NULL,
    year    integer NOT NULL,
    value   bigint NOT NULL,
    PRIMARY KEY (prefix, year)
);

CREATE TABLE aid_requests (
    id                          uuid PRIMARY KEY,
    reference                   text NOT NULL UNIQUE,
    organization_id             uuid NOT NULL REFERENCES organizations(id),
    created_by_user_id          uuid REFERENCES users(id),
    client_id                   uuid UNIQUE,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    status                      text NOT NULL CHECK (status IN (
                                    'draft', 'submitted', 'under_review', 'verified', 'assigned',
                                    'in_progress', 'partially_resolved', 'resolved', 'cancelled',
                                    'expired', 'duplicate')),
    category                    text NOT NULL REFERENCES categories(code),
    urgency                     text NOT NULL CHECK (urgency IN ('critical', 'high', 'normal', 'low')),
    title                       text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    description                 text NOT NULL DEFAULT '' CHECK (char_length(description) <= 4000),
    estimated_people_affected   integer CHECK (estimated_people_affected BETWEEN 0 AND 100000),
    requested_quantity          integer CHECK (requested_quantity BETWEEN 0 AND 10000000),
    requested_unit              text NOT NULL DEFAULT '' CHECK (char_length(requested_unit) <= 40),
    requested_by_time           timestamptz,
    location_mode               text NOT NULL DEFAULT 'none'
                                CHECK (location_mode IN ('none', 'area_only', 'approximate', 'protected_exact')),
    area_label                  text NOT NULL DEFAULT '' CHECK (char_length(area_label) <= 160),
    approx_lat                  double precision CHECK (approx_lat BETWEEN -90 AND 90),
    approx_lon                  double precision CHECK (approx_lon BETWEEN -180 AND 180),
    approx_decimals             smallint,
    exact_location_sealed       bytea,
    accessibility_notes         text NOT NULL DEFAULT '' CHECK (char_length(accessibility_notes) <= 1000),
    contact_visibility          text NOT NULL DEFAULT 'coordinators_only'
                                CHECK (contact_visibility IN ('none', 'coordinators_only', 'assigned_responders')),
    contact_method              text NOT NULL DEFAULT 'none'
                                CHECK (contact_method IN ('none', 'phone', 'messenger', 'email', 'in_person', 'via_shelter_desk', 'other')),
    contact_details_sealed      bytea,
    sensitive_data_flag         boolean NOT NULL DEFAULT false,
    requires_formal_authorization boolean NOT NULL DEFAULT false,
    review_status               text NOT NULL DEFAULT 'pending'
                                CHECK (review_status IN ('pending', 'in_review', 'reviewed', 'needs_information')),
    verification_level          text NOT NULL DEFAULT 'unverified'
                                CHECK (verification_level IN ('unverified', 'self_reported', 'coordinator_confirmed', 'field_confirmed')),
    assigned_team               text NOT NULL DEFAULT '' CHECK (char_length(assigned_team) <= 120),
    assigned_user_id            uuid REFERENCES users(id),
    resolution_summary          text NOT NULL DEFAULT '' CHECK (char_length(resolution_summary) <= 2000),
    closed_at                   timestamptz,
    expiry_at                   timestamptz,
    tags                        text[] NOT NULL DEFAULT '{}',
    duplicate_of_request_id     uuid REFERENCES aid_requests(id),
    version                     integer NOT NULL DEFAULT 1,
    redacted_at                 timestamptz,
    deleted_at                  timestamptz,
    CONSTRAINT aid_requests_duplicate_requires_canonical
        CHECK (status <> 'duplicate' OR duplicate_of_request_id IS NOT NULL),
    CONSTRAINT aid_requests_duplicate_not_self
        CHECK (duplicate_of_request_id IS NULL OR duplicate_of_request_id <> id),
    CONSTRAINT aid_requests_approx_pair
        CHECK ((approx_lat IS NULL) = (approx_lon IS NULL)),
    CONSTRAINT aid_requests_contact_none
        CHECK (contact_visibility <> 'none' OR contact_details_sealed IS NULL)
);
CREATE INDEX aid_requests_org_status_idx ON aid_requests (organization_id, status) WHERE deleted_at IS NULL;
CREATE INDEX aid_requests_creator_idx ON aid_requests (created_by_user_id);
CREATE INDEX aid_requests_updated_idx ON aid_requests (updated_at);

CREATE TABLE offers (
    id                          uuid PRIMARY KEY,
    reference                   text NOT NULL UNIQUE,
    organization_id             uuid NOT NULL REFERENCES organizations(id),
    created_by_user_id          uuid REFERENCES users(id),
    client_id                   uuid UNIQUE,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    status                      text NOT NULL CHECK (status IN (
                                    'draft', 'available', 'partially_allocated', 'fully_allocated',
                                    'paused', 'expired', 'cancelled')),
    category                    text NOT NULL REFERENCES categories(code),
    title                       text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    description                 text NOT NULL DEFAULT '' CHECK (char_length(description) <= 4000),
    quantity_available          integer NOT NULL CHECK (quantity_available BETWEEN 0 AND 10000000),
    unit                        text NOT NULL DEFAULT '' CHECK (char_length(unit) <= 40),
    availability_start          timestamptz,
    availability_end            timestamptz,
    pickup_or_delivery_mode     text NOT NULL DEFAULT 'pickup'
                                CHECK (pickup_or_delivery_mode IN ('pickup', 'delivery', 'either', 'on_site')),
    location_mode               text NOT NULL DEFAULT 'none'
                                CHECK (location_mode IN ('none', 'area_only', 'approximate', 'protected_exact')),
    area_label                  text NOT NULL DEFAULT '' CHECK (char_length(area_label) <= 160),
    approx_lat                  double precision CHECK (approx_lat BETWEEN -90 AND 90),
    approx_lon                  double precision CHECK (approx_lon BETWEEN -180 AND 180),
    approx_decimals             smallint,
    exact_location_sealed       bytea,
    accessibility_notes         text NOT NULL DEFAULT '' CHECK (char_length(accessibility_notes) <= 1000),
    restrictions                text NOT NULL DEFAULT '' CHECK (char_length(restrictions) <= 1000),
    contact_visibility          text NOT NULL DEFAULT 'coordinators_only'
                                CHECK (contact_visibility IN ('none', 'coordinators_only', 'assigned_responders')),
    contact_method              text NOT NULL DEFAULT 'none'
                                CHECK (contact_method IN ('none', 'phone', 'messenger', 'email', 'in_person', 'via_shelter_desk', 'other')),
    contact_details_sealed      bytea,
    verification_level          text NOT NULL DEFAULT 'unverified'
                                CHECK (verification_level IN ('unverified', 'self_reported', 'coordinator_confirmed', 'field_confirmed')),
    assigned_quantity           integer NOT NULL DEFAULT 0,
    remaining_quantity          integer GENERATED ALWAYS AS (quantity_available - assigned_quantity) STORED,
    tags                        text[] NOT NULL DEFAULT '{}',
    version                     integer NOT NULL DEFAULT 1,
    redacted_at                 timestamptz,
    deleted_at                  timestamptz,
    -- Hard guarantee against over-allocation, independent of application logic.
    CONSTRAINT offers_allocation_bounds
        CHECK (assigned_quantity >= 0 AND assigned_quantity <= quantity_available),
    CONSTRAINT offers_availability_window
        CHECK (availability_start IS NULL OR availability_end IS NULL OR availability_end >= availability_start),
    CONSTRAINT offers_approx_pair
        CHECK ((approx_lat IS NULL) = (approx_lon IS NULL)),
    CONSTRAINT offers_contact_none
        CHECK (contact_visibility <> 'none' OR contact_details_sealed IS NULL)
);
CREATE INDEX offers_org_status_idx ON offers (organization_id, status) WHERE deleted_at IS NULL;
CREATE INDEX offers_creator_idx ON offers (created_by_user_id);
CREATE INDEX offers_updated_idx ON offers (updated_at);

CREATE TABLE assignments (
    id                                  uuid PRIMARY KEY,
    organization_id                     uuid NOT NULL REFERENCES organizations(id),
    client_id                           uuid UNIQUE,
    request_id                          uuid NOT NULL REFERENCES aid_requests(id),
    offer_id                            uuid REFERENCES offers(id),
    volunteer_user_id                   uuid REFERENCES users(id),
    team_label                          text NOT NULL DEFAULT '' CHECK (char_length(team_label) <= 120),
    assigned_by_user_id                 uuid NOT NULL REFERENCES users(id),
    assigned_at                         timestamptz NOT NULL DEFAULT now(),
    updated_at                          timestamptz NOT NULL DEFAULT now(),
    status                              text NOT NULL CHECK (status IN (
                                            'proposed', 'accepted', 'declined', 'in_progress', 'delivered',
                                            'partially_delivered', 'unable_to_complete', 'cancelled')),
    quantity_assigned                   integer NOT NULL DEFAULT 0 CHECK (quantity_assigned BETWEEN 0 AND 10000000),
    unit                                text NOT NULL DEFAULT '' CHECK (char_length(unit) <= 40),
    allocation_released                 boolean NOT NULL DEFAULT false,
    instructions                        text NOT NULL DEFAULT '' CHECK (char_length(instructions) <= 2000),
    protected_contact_access_granted    boolean NOT NULL DEFAULT false,
    pickup_location_access_granted      boolean NOT NULL DEFAULT false,
    destination_location_access_granted boolean NOT NULL DEFAULT false,
    eta_text                            text NOT NULL DEFAULT '' CHECK (char_length(eta_text) <= 120),
    accepted_at                         timestamptz,
    started_at                          timestamptz,
    completed_at                        timestamptz,
    handover_status                     text NOT NULL DEFAULT 'not_started'
                                        CHECK (handover_status IN ('not_started', 'handed_over', 'received', 'not_applicable')),
    handover_notes                      text NOT NULL DEFAULT '' CHECK (char_length(handover_notes) <= 2000),
    completion_evidence_type            text NOT NULL DEFAULT 'no_evidence'
                                        CHECK (completion_evidence_type IN (
                                            'coordinator_confirmation', 'requester_confirmation',
                                            'volunteer_confirmation', 'inventory_handover_record', 'no_evidence')),
    completion_evidence_reference       text NOT NULL DEFAULT '' CHECK (char_length(completion_evidence_reference) <= 200),
    cancellation_reason                 text NOT NULL DEFAULT '' CHECK (char_length(cancellation_reason) <= 1000),
    version                             integer NOT NULL DEFAULT 1,
    CONSTRAINT assignments_target_present
        CHECK (offer_id IS NOT NULL OR volunteer_user_id IS NOT NULL OR team_label <> '')
);
CREATE INDEX assignments_request_idx ON assignments (request_id);
CREATE INDEX assignments_offer_idx ON assignments (offer_id);
CREATE INDEX assignments_volunteer_idx ON assignments (volunteer_user_id);
CREATE INDEX assignments_updated_idx ON assignments (updated_at);

CREATE TABLE notes (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id),
    client_id       uuid UNIQUE,
    request_id      uuid REFERENCES aid_requests(id),
    offer_id        uuid REFERENCES offers(id),
    author_user_id  uuid REFERENCES users(id),
    visibility      text NOT NULL CHECK (visibility IN ('internal', 'responders', 'shared')),
    is_sensitive    boolean NOT NULL DEFAULT false,
    body            text NOT NULL CHECK (char_length(body) <= 4000),
    created_at      timestamptz NOT NULL DEFAULT now(),
    redacted_at     timestamptz,
    CONSTRAINT notes_single_parent CHECK ((request_id IS NULL) <> (offer_id IS NULL))
);
CREATE INDEX notes_request_idx ON notes (request_id);
CREATE INDEX notes_offer_idx ON notes (offer_id);

CREATE TABLE audit_events (
    id              bigint PRIMARY KEY,
    occurred_at     timestamptz NOT NULL,
    organization_id uuid,
    actor_user_id   uuid,
    actor_roles     text[] NOT NULL DEFAULT '{}',
    action          text NOT NULL,
    entity_type     text NOT NULL DEFAULT '',
    entity_id       uuid,
    request_id      uuid,
    from_status     text NOT NULL DEFAULT '',
    to_status       text NOT NULL DEFAULT '',
    reason          text NOT NULL DEFAULT '',
    metadata        jsonb NOT NULL DEFAULT '{}'::jsonb,
    visibility      text NOT NULL CHECK (visibility IN ('shared', 'operational', 'internal', 'system')),
    prev_hash       bytea NOT NULL,
    hash            bytea NOT NULL
);
CREATE INDEX audit_events_entity_idx ON audit_events (entity_type, entity_id);
CREATE INDEX audit_events_request_idx ON audit_events (request_id);
CREATE INDEX audit_events_action_idx ON audit_events (action);

CREATE FUNCTION reliefmesh_audit_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only (% rejected)', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$;

CREATE TRIGGER audit_events_no_update_delete
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION reliefmesh_audit_immutable();

CREATE TRIGGER audit_events_no_truncate
    BEFORE TRUNCATE ON audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION reliefmesh_audit_immutable();

CREATE TABLE sync_operations (
    op_id        uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users(id),
    op_type      text NOT NULL,
    status       text NOT NULL,
    result       jsonb NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sync_operations_user_idx ON sync_operations (user_id, received_at);
