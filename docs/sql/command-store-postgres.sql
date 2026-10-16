-- Apply explicitly in the application's selected schema. Never run automatically.
CREATE TABLE goquent_commands_v1 (
 scope_hash BYTEA NOT NULL,
 operation_id BYTEA NOT NULL,
 format_version INTEGER NOT NULL,
 canonical_scope BYTEA NOT NULL,
 command_input BYTEA NOT NULL,
 fingerprint BYTEA NOT NULL,
 status INTEGER NOT NULL,
 result BYTEA NOT NULL,
 PRIMARY KEY (scope_hash, operation_id)
);
