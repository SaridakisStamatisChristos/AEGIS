DROP TRIGGER IF EXISTS event_hash_validation_trigger ON events;
DROP FUNCTION IF EXISTS verify_event_hash();

DROP TRIGGER IF EXISTS event_immutability_trigger ON events;
DROP FUNCTION IF EXISTS prevent_event_deletion();

DROP TRIGGER IF EXISTS policy_immutability_trigger ON policies;
DROP FUNCTION IF EXISTS prevent_policy_spec_modification();

DROP TRIGGER IF EXISTS key_status_audit_trigger ON signing_keys;
DROP FUNCTION IF EXISTS audit_key_status_changes();

DROP TRIGGER IF EXISTS user_role_audit_trigger ON users;
DROP FUNCTION IF EXISTS audit_user_role_changes();

DROP TRIGGER IF EXISTS policy_audit_trigger ON policies;
DROP FUNCTION IF EXISTS audit_policy_changes();
