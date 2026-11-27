
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA procurement;

CREATE TABLE procurement.requests (
    id UUID PRIMARY KEY,
    buyer_id BIGINT,
    status TEXT NOT NULL DEFAULT 'CLARIFYING',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT requests_status_check CHECK (status IN ('CLARIFYING', 'REQUIREMENT_READY', 'MATCHING')),
    CONSTRAINT requests_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX requests_buyer ON procurement.requests (buyer_id, created_at DESC) WHERE buyer_id IS NOT NULL;

CREATE TABLE procurement.request_revisions (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    revision_no INT NOT NULL,
    parent_revision_id UUID REFERENCES procurement.request_revisions (id),
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    item TEXT,
    item_resolution_kind TEXT,
    item_resolution_label TEXT,
    item_resolution_category_id UUID,
    quantity INT,
    unit TEXT,
    packaging TEXT,
    budget_per_unit BIGINT,
    delivery_city TEXT,
    delivery_date DATE,
    exact_match_required BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT request_revisions_no_check CHECK (revision_no >= 1),
    CONSTRAINT request_revisions_status_check CHECK (status IN ('ACTIVE', 'SUPERSEDED')),
    CONSTRAINT request_revisions_resolution_check CHECK (
        (item_resolution_kind IS NULL) = (item_resolution_label IS NULL)
        AND (item_resolution_kind IS NULL OR item_resolution_kind IN ('CATEGORY_CONFIRMED', 'BUYER_DESCRIBED'))
    ),
    CONSTRAINT request_revisions_resolution_category_check CHECK (
        (item_resolution_category_id IS NOT NULL) = (item_resolution_kind IS NOT DISTINCT FROM 'CATEGORY_CONFIRMED')
        AND (item_resolution_category_id IS NULL OR item_resolution_category_id <> '00000000-0000-0000-0000-000000000000')
    ),
    CONSTRAINT request_revisions_item_check CHECK (char_length(item) >= 1 AND char_length(item) <= 200 AND item !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_resolution_label_check CHECK (char_length(item_resolution_label) >= 1 AND char_length(item_resolution_label) <= 200 AND item_resolution_label !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_unit_check CHECK (char_length(unit) >= 1 AND char_length(unit) <= 40 AND unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_packaging_check CHECK (char_length(packaging) >= 1 AND char_length(packaging) <= 200 AND packaging !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_delivery_city_check CHECK (char_length(delivery_city) >= 1 AND char_length(delivery_city) <= 100 AND delivery_city !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT request_revisions_quantity_check CHECK (quantity > 0),
    CONSTRAINT request_revisions_budget_check CHECK (budget_per_unit >= 0)
);
CREATE UNIQUE INDEX request_revisions_no ON procurement.request_revisions (request_id, revision_no);
CREATE UNIQUE INDEX request_revisions_active ON procurement.request_revisions (request_id) WHERE status = 'ACTIVE';

CREATE TABLE procurement.conversations (
    id UUID PRIMARY KEY,
    buyer_id BIGINT NOT NULL,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    title_source TEXT NOT NULL DEFAULT 'AUTO',
    last_message_preview TEXT NOT NULL DEFAULT '',
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT conversations_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT conversations_title_source_check CHECK (title_source IN ('AUTO', 'USER')),
    CONSTRAINT conversations_title_check CHECK (char_length(title) <= 40 AND title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT conversations_preview_check CHECK (char_length(last_message_preview) <= 200 AND last_message_preview !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE UNIQUE INDEX conversations_request ON procurement.conversations (request_id);
CREATE INDEX conversations_buyer_active ON procurement.conversations (buyer_id, updated_at DESC, id DESC) WHERE deleted_at IS NULL;

CREATE TABLE procurement.turns (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES procurement.conversations (id) ON DELETE CASCADE,
    buyer_id BIGINT NOT NULL,
    result_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    buyer_message TEXT NOT NULL DEFAULT '',
    assistant_reply TEXT NOT NULL DEFAULT '',
    steps JSONB NOT NULL DEFAULT '[]',
    recorded_this_turn JSONB NOT NULL DEFAULT '[]',
    optional_streak INTEGER NOT NULL DEFAULT 0,
    skipped_slots TEXT[] NOT NULL DEFAULT '{}',
    turn_result JSONB NOT NULL,
    command_slot TEXT,
    client_request_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT turns_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT turns_source_check CHECK (source IN ('LLM_EXTRACT', 'ANSWER', 'REWIND')),
    CONSTRAINT turns_buyer_message_check CHECK (char_length(buyer_message) <= 2000 AND buyer_message !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT turns_assistant_reply_check CHECK (char_length(assistant_reply) <= 4000 AND assistant_reply !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT turns_steps_check CHECK (jsonb_typeof(steps) = 'array'),
    CONSTRAINT turns_recorded_check CHECK (jsonb_typeof(recorded_this_turn) = 'array'),
    CONSTRAINT turns_turn_result_check CHECK (jsonb_typeof(turn_result) = 'object'),
    CONSTRAINT turns_optional_streak_check CHECK (optional_streak >= 0),
    CONSTRAINT turns_skipped_slots_check CHECK (skipped_slots <@ ARRAY['PACKAGING', 'BUDGET_PER_UNIT']),
    CONSTRAINT turns_command_slot_check CHECK (
        command_slot IS NOT NULL AND command_slot IN ('ITEM', 'QUANTITY', 'PACKAGING', 'BUDGET_PER_UNIT', 'DELIVERY_CITY', 'DELIVERY_DATE')
        AND source IN ('ANSWER', 'REWIND')
        OR command_slot IS NULL AND source IN ('LLM_EXTRACT', 'ANSWER')
    )
);
CREATE UNIQUE INDEX turns_idempotency ON procurement.turns (buyer_id, client_request_id);
CREATE INDEX turns_request ON procurement.turns (request_id, created_at DESC);

CREATE TABLE procurement.candidate_sets (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    buyer_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT candidate_sets_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX candidate_sets_request ON procurement.candidate_sets (request_id, created_at DESC);

CREATE TABLE procurement.candidates (
    id UUID PRIMARY KEY,
    candidate_set_id UUID NOT NULL REFERENCES procurement.candidate_sets (id) ON DELETE CASCADE,
    store_id BIGINT NOT NULL,
    source_id TEXT NOT NULL,
    rank INT NOT NULL DEFAULT 0,
    selected BOOLEAN NOT NULL DEFAULT FALSE,
    selected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT candidates_store_check CHECK (store_id > 0),
    CONSTRAINT candidates_source_check CHECK (char_length(source_id) >= 1),
    CONSTRAINT candidates_selected_check CHECK (selected = (selected_at IS NOT NULL))
);
CREATE UNIQUE INDEX candidates_set_store ON procurement.candidates (candidate_set_id, store_id);
CREATE INDEX candidates_set_rank ON procurement.candidates (candidate_set_id, rank);

CREATE TABLE procurement.rfq_publication_approvals (
    id UUID PRIMARY KEY,
    version INT NOT NULL DEFAULT 1,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    range_lower_amount BIGINT NOT NULL,
    range_upper_amount BIGINT NOT NULL,
    buyer_note TEXT NOT NULL DEFAULT '',
    candidate_set_id UUID NOT NULL REFERENCES procurement.candidate_sets (id) ON DELETE RESTRICT,
    response_deadline DATE,
    state TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    client_request_id UUID NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT rfq_publication_approvals_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT rfq_publication_approvals_version_check CHECK (version >= 1),
    CONSTRAINT rfq_publication_approvals_state_check CHECK (state IN ('PENDING', 'EXECUTING', 'SUCCEEDED')),
    CONSTRAINT rfq_publication_approvals_schema_check CHECK (char_length(schema_version) >= 1),
    CONSTRAINT rfq_publication_approvals_range_check CHECK (range_lower_amount > 0 AND range_upper_amount >= range_lower_amount),
    CONSTRAINT rfq_publication_approvals_note_check CHECK (char_length(buyer_note) <= 500 AND buyer_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE UNIQUE INDEX rfq_publication_approvals_idempotency ON procurement.rfq_publication_approvals (buyer_id, client_request_id);
CREATE INDEX rfq_publication_approvals_request ON procurement.rfq_publication_approvals (request_id, created_at DESC);

CREATE TABLE procurement.rfq_publication_approval_recipients (
    approval_id UUID NOT NULL REFERENCES procurement.rfq_publication_approvals (id) ON DELETE CASCADE,
    position INT NOT NULL,
    store_id BIGINT NOT NULL,
    PRIMARY KEY (approval_id, position),
    CONSTRAINT rfq_publication_approval_recipients_position_check CHECK (position >= 0),
    CONSTRAINT rfq_publication_approval_recipients_store_check CHECK (store_id > 0)
);
CREATE UNIQUE INDEX rfq_publication_approval_recipients_store ON procurement.rfq_publication_approval_recipients (approval_id, store_id);

CREATE TABLE procurement.rfq_publications (
    id UUID PRIMARY KEY,
    version INT NOT NULL DEFAULT 1,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    request_revision_id UUID NOT NULL REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    approval_id UUID NOT NULL REFERENCES procurement.rfq_publication_approvals (id) ON DELETE CASCADE,
    approval_version INT NOT NULL,
    state TEXT NOT NULL DEFAULT 'PUBLISHED',
    schema_version TEXT NOT NULL,
    client_request_id UUID NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT rfq_publications_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT rfq_publications_version_check CHECK (version >= 1),
    CONSTRAINT rfq_publications_state_check CHECK (state IN ('PUBLISHED')),
    CONSTRAINT rfq_publications_approval_version_check CHECK (approval_version >= 1),
    CONSTRAINT rfq_publications_schema_check CHECK (char_length(schema_version) >= 1)
);
CREATE UNIQUE INDEX rfq_publications_approval ON procurement.rfq_publications (approval_id);
CREATE UNIQUE INDEX rfq_publications_idempotency ON procurement.rfq_publications (buyer_id, client_request_id);
CREATE INDEX rfq_publications_request ON procurement.rfq_publications (request_id, created_at DESC);

CREATE TABLE procurement.rfq_publication_invites (
    id UUID PRIMARY KEY,
    publication_id UUID NOT NULL REFERENCES procurement.rfq_publications (id) ON DELETE CASCADE,
    position INT NOT NULL,
    store_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT rfq_publication_invites_position_check CHECK (position >= 0),
    CONSTRAINT rfq_publication_invites_store_check CHECK (store_id > 0)
);
CREATE UNIQUE INDEX rfq_publication_invites_store ON procurement.rfq_publication_invites (publication_id, store_id);
CREATE UNIQUE INDEX rfq_publication_invites_position ON procurement.rfq_publication_invites (publication_id, position);

CREATE TABLE procurement.seller_responses (
    id UUID PRIMARY KEY,
    publication_id UUID NOT NULL REFERENCES procurement.rfq_publications (id) ON DELETE CASCADE,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    store_id BIGINT NOT NULL,
    latest_version INT NOT NULL DEFAULT 0,
    withdrawn_version INT,
    withdrawn_at TIMESTAMPTZ,
    write_closed_at TIMESTAMPTZ,
    write_closed_reason TEXT,
    withdrawn_reason_code TEXT,
    write_closed_reason_code TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_responses_invite_fk FOREIGN KEY (publication_id, store_id) REFERENCES procurement.rfq_publication_invites (publication_id, store_id),
    CONSTRAINT seller_responses_store_check CHECK (store_id > 0),
    CONSTRAINT seller_responses_latest_check CHECK (latest_version >= 0),
    CONSTRAINT seller_responses_withdrawn_pair CHECK ((withdrawn_version IS NULL) = (withdrawn_at IS NULL)),
    CONSTRAINT seller_responses_write_closed_pair CHECK ((write_closed_at IS NULL) = (write_closed_reason IS NULL)),
    CONSTRAINT seller_responses_write_closed_reason CHECK (write_closed_reason IS NULL OR write_closed_reason IN ('DECLINED')),
    CONSTRAINT seller_responses_withdrawn_reason_pair CHECK (withdrawn_reason_code IS NULL OR withdrawn_version IS NOT NULL),
    CONSTRAINT seller_responses_write_closed_reason_pair CHECK (write_closed_reason_code IS NULL OR write_closed_reason IS NOT NULL),
    CONSTRAINT seller_responses_withdrawn_reason_code_check CHECK (withdrawn_reason_code IS NULL OR (char_length(withdrawn_reason_code) >= 1 AND char_length(withdrawn_reason_code) <= 64 AND withdrawn_reason_code !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT seller_responses_write_closed_reason_code_check CHECK (write_closed_reason_code IS NULL OR (char_length(write_closed_reason_code) >= 1 AND char_length(write_closed_reason_code) <= 64 AND write_closed_reason_code !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);
CREATE UNIQUE INDEX seller_responses_publication_store ON procurement.seller_responses (publication_id, store_id);
CREATE INDEX seller_responses_request_submitted ON procurement.seller_responses (request_id, created_at DESC) WHERE latest_version > 0;

CREATE TABLE procurement.seller_response_versions (
    id UUID PRIMARY KEY,
    seller_response_id UUID NOT NULL REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    version INT NOT NULL,
    response_type TEXT NOT NULL,
    spec_note TEXT NOT NULL DEFAULT '',
    unit_basis_quantity NUMERIC(14, 3) NOT NULL,
    unit_basis_unit TEXT NOT NULL,
    offered_quantity NUMERIC(14, 3) NOT NULL,
    offered_quantity_unit TEXT NOT NULL,
    conversion_basis TEXT NOT NULL,
    currency TEXT NOT NULL,
    goods_amount BIGINT NOT NULL,
    shipping_amount BIGINT NOT NULL,
    tax_amount BIGINT NOT NULL,
    tax_inclusion_basis TEXT NOT NULL,
    total_amount BIGINT NOT NULL,
    lead_time_days INT,
    promised_delivery_at TIMESTAMPTZ,
    valid_until TIMESTAMPTZ NOT NULL,
    terms_note TEXT NOT NULL DEFAULT '',
    deviation_disclosure TEXT NOT NULL DEFAULT '',
    publication_version INT NOT NULL,
    client_request_id UUID NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_response_versions_version_check CHECK (version >= 1),
    CONSTRAINT seller_response_versions_type_check CHECK (response_type IN ('MANUAL_QUOTE', 'ACCEPT_NON_PRICE_TERMS_WITH_QUOTE')),
    CONSTRAINT seller_response_versions_tax_basis_check CHECK (tax_inclusion_basis IN ('TAX_INCLUDED', 'TAX_EXCLUDED')),
    CONSTRAINT seller_response_versions_amounts_check CHECK (goods_amount >= 0 AND shipping_amount >= 0 AND tax_amount >= 0),
    CONSTRAINT seller_response_versions_total_check CHECK (total_amount = goods_amount + shipping_amount + tax_amount),
    CONSTRAINT seller_response_versions_quantities_check CHECK (unit_basis_quantity > 0 AND offered_quantity > 0),
    CONSTRAINT seller_response_versions_publication_version_check CHECK (publication_version >= 1),
    CONSTRAINT seller_response_versions_spec_note_check CHECK (char_length(spec_note) <= 1000 AND spec_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_terms_check CHECK (char_length(terms_note) <= 1000 AND terms_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_deviation_check CHECK (char_length(deviation_disclosure) <= 1000 AND deviation_disclosure !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_unit_basis_unit_check CHECK (char_length(unit_basis_unit) >= 1 AND char_length(unit_basis_unit) <= 40 AND unit_basis_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_offered_unit_check CHECK (char_length(offered_quantity_unit) >= 1 AND char_length(offered_quantity_unit) <= 40 AND offered_quantity_unit !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT seller_response_versions_conversion_basis_check CHECK (char_length(conversion_basis) <= 500 AND conversion_basis !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE UNIQUE INDEX seller_response_versions_version ON procurement.seller_response_versions (seller_response_id, version);
CREATE UNIQUE INDEX seller_response_versions_idempotency ON procurement.seller_response_versions (seller_response_id, client_request_id);

CREATE TABLE procurement.seller_response_drafts (
    seller_response_id UUID PRIMARY KEY REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_response_drafts_payload_check CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT seller_response_drafts_payload_size_check CHECK (octet_length(payload::text) <= 16384),
    CONSTRAINT seller_response_drafts_version_check CHECK (version >= 1)
);

CREATE TABLE procurement.buyer_shortlist_entries (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    seller_response_id UUID NOT NULL REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    seller_response_version INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT buyer_shortlist_entries_version_check CHECK (seller_response_version >= 1)
);
CREATE UNIQUE INDEX buyer_shortlist_entries_response ON procurement.buyer_shortlist_entries (request_id, seller_response_id);

CREATE TABLE procurement.buyer_quote_acceptances (
    id UUID PRIMARY KEY,
    buyer_id BIGINT,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    publication_id UUID NOT NULL REFERENCES procurement.rfq_publications (id) ON DELETE CASCADE,
    seller_response_id UUID NOT NULL REFERENCES procurement.seller_responses (id) ON DELETE CASCADE,
    seller_response_version INT NOT NULL,
    seller_response_payload_hash TEXT NOT NULL,
    seller_facing_hash TEXT NOT NULL,
    store_id BIGINT NOT NULL,
    currency TEXT NOT NULL,
    goods_amount BIGINT NOT NULL,
    shipping_amount BIGINT NOT NULL,
    tax_amount BIGINT NOT NULL,
    accepted_deal_price BIGINT NOT NULL,
    state TEXT NOT NULL DEFAULT 'ACTIVE',
    order_provisioning_state TEXT NOT NULL DEFAULT 'CREATING_ORDER',
    order_failure_reason TEXT,
    order_no TEXT,
    payment_deadline TIMESTAMPTZ,
    order_ref BIGINT NOT NULL GENERATED ALWAYS AS IDENTITY UNIQUE,
    order_job_id BIGINT,
    client_request_id UUID NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT buyer_quote_acceptances_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT buyer_quote_acceptances_version_check CHECK (seller_response_version >= 1),
    CONSTRAINT buyer_quote_acceptances_store_check CHECK (store_id > 0),
    CONSTRAINT buyer_quote_acceptances_state_check CHECK (state IN ('ACTIVE', 'ORDER_FAILED')),
    CONSTRAINT buyer_quote_acceptances_provisioning_check CHECK (order_provisioning_state IN ('CREATING_ORDER', 'ORDER_CREATED', 'ORDER_FAILED')),
    CONSTRAINT buyer_quote_acceptances_failed_check CHECK ((state = 'ORDER_FAILED') = (order_provisioning_state = 'ORDER_FAILED')),
    CONSTRAINT buyer_quote_acceptances_order_no_check CHECK ((order_provisioning_state = 'ORDER_CREATED') = (order_no IS NOT NULL)),
    CONSTRAINT buyer_quote_acceptances_amounts_check CHECK (goods_amount >= 0 AND shipping_amount >= 0 AND tax_amount >= 0),
    CONSTRAINT buyer_quote_acceptances_price_check CHECK (accepted_deal_price = goods_amount + shipping_amount + tax_amount)
);
CREATE UNIQUE INDEX buyer_quote_acceptances_idempotency ON procurement.buyer_quote_acceptances (buyer_id, client_request_id);
CREATE UNIQUE INDEX buyer_quote_acceptances_live ON procurement.buyer_quote_acceptances (request_id) WHERE state = 'ACTIVE';
CREATE UNIQUE INDEX buyer_quote_acceptances_order_no ON procurement.buyer_quote_acceptances (order_no) WHERE order_no IS NOT NULL;
CREATE INDEX buyer_quote_acceptances_response ON procurement.buyer_quote_acceptances (seller_response_id, seller_response_version);
CREATE INDEX buyer_quote_acceptances_creating ON procurement.buyer_quote_acceptances (id) WHERE order_provisioning_state = 'CREATING_ORDER';

CREATE TABLE procurement.agent_runs (
    id UUID PRIMARY KEY,
    buyer_id BIGINT NOT NULL,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES procurement.conversations (id) ON DELETE CASCADE,
    interrupt_id UUID,
    candidate_set_id UUID REFERENCES procurement.candidate_sets (id) ON DELETE CASCADE,
    run_type TEXT NOT NULL,
    status TEXT NOT NULL,
    failure_reason TEXT,
    result_revision_id UUID REFERENCES procurement.request_revisions (id) ON DELETE CASCADE,
    last_event_seq BIGINT NOT NULL DEFAULT 0,
    min_available_seq BIGINT NOT NULL DEFAULT 1,
    client_request_id UUID NOT NULL,
    job_id BIGINT,
    quota_period_start TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_runs_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT agent_runs_type_check CHECK (run_type IN ('TURN', 'CANDIDATE_SEARCH', 'CLARIFICATION_RESUME')),
    CONSTRAINT agent_runs_status_check CHECK (status IN ('QUEUED', 'RUNNING', 'WAITING_FOR_INPUT', 'CANCEL_REQUESTED', 'SUCCEEDED', 'FAILED', 'CANCELLED')),
    CONSTRAINT agent_runs_failure_check CHECK (failure_reason IS NULL OR failure_reason IN ('WORKER_LOST', 'SEARCH_FAILED', 'TURN_PIPELINE_ERROR', 'MODEL_UNAVAILABLE', 'ITEM_CATEGORY_UNAVAILABLE', 'ANSWER_REJECTED')),
    CONSTRAINT agent_runs_seq_check CHECK (last_event_seq >= 0 AND min_available_seq >= 1),
    CONSTRAINT agent_runs_interrupt_check CHECK ((run_type = 'CLARIFICATION_RESUME') = (interrupt_id IS NOT NULL)),
    CONSTRAINT agent_runs_quota_check CHECK ((run_type = 'TURN') = (quota_period_start IS NOT NULL))
);
CREATE UNIQUE INDEX agent_runs_idempotency ON procurement.agent_runs (buyer_id, client_request_id);
CREATE INDEX agent_runs_interrupt ON procurement.agent_runs (interrupt_id) WHERE interrupt_id IS NOT NULL;
CREATE INDEX agent_runs_request ON procurement.agent_runs (request_id, created_at DESC);
CREATE INDEX agent_runs_unfinished ON procurement.agent_runs (id) WHERE status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED');
CREATE INDEX agent_runs_buyer_created ON procurement.agent_runs (buyer_id, created_at DESC);
CREATE INDEX agent_runs_buyer_active ON procurement.agent_runs (buyer_id) WHERE status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED');
CREATE INDEX agent_runs_event_expiry ON procurement.agent_runs (completed_at) WHERE completed_at IS NOT NULL AND min_available_seq <= last_event_seq;

CREATE TABLE procurement.agent_run_inputs (
    run_id UUID PRIMARY KEY REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    buyer_message TEXT NOT NULL,
    option_id TEXT,
    locale TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_run_inputs_locale_check CHECK (locale IN ('ZH_TW', 'EN')),
    CONSTRAINT agent_run_inputs_message_check CHECK (char_length(buyer_message) <= 2000 AND buyer_message !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT agent_run_inputs_option_check CHECK (option_id IS NULL OR (char_length(option_id) >= 1 AND char_length(option_id) <= 200 AND option_id !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT agent_run_inputs_answer_check CHECK ((option_id IS NULL) = (char_length(buyer_message) >= 1))
);

CREATE TABLE procurement.agent_run_events (
    run_id UUID NOT NULL REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (run_id, seq),
    CONSTRAINT agent_run_events_seq_check CHECK (seq >= 1),
    CONSTRAINT agent_run_events_type_check CHECK (char_length(event_type) >= 1),
    CONSTRAINT agent_run_events_payload_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE TABLE procurement.agent_run_interrupts (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES procurement.agent_runs (id) ON DELETE CASCADE,
    request_id UUID NOT NULL REFERENCES procurement.requests (id) ON DELETE CASCADE,
    interrupt_kind TEXT NOT NULL,
    status TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    facts_as_of TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    CONSTRAINT agent_run_interrupts_kind_check CHECK (interrupt_kind IN ('CLARIFICATION')),
    CONSTRAINT agent_run_interrupts_status_check CHECK (status IN ('PENDING', 'RESOLVED', 'CANCELLED')),
    CONSTRAINT agent_run_interrupts_payload_check CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT agent_run_interrupts_closed_check CHECK ((status = 'PENDING') = (closed_at IS NULL))
);
CREATE INDEX agent_run_interrupts_run ON procurement.agent_run_interrupts (run_id);
CREATE UNIQUE INDEX agent_run_interrupts_pending ON procurement.agent_run_interrupts (request_id) WHERE status = 'PENDING';
ALTER TABLE procurement.agent_runs ADD CONSTRAINT agent_runs_interrupt_fkey FOREIGN KEY (interrupt_id) REFERENCES procurement.agent_run_interrupts (id) ON DELETE CASCADE;

CREATE TABLE procurement.usage (
    buyer_id BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    used INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, period_start),
    CONSTRAINT usage_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT usage_used_check CHECK (used >= 0)
);
