-- Apply explicitly in the application's selected database. Never run automatically.
CREATE TABLE goquent_commands_v1 (
 scope_hash BINARY(32) NOT NULL,
 operation_id BINARY(36) NOT NULL,
 format_version INT NOT NULL,
 canonical_scope MEDIUMBLOB NOT NULL,
 command_input MEDIUMBLOB NOT NULL,
 fingerprint BINARY(32) NOT NULL,
 status INT NOT NULL,
 result MEDIUMBLOB NOT NULL,
 PRIMARY KEY (scope_hash, operation_id)
) ENGINE=InnoDB;
