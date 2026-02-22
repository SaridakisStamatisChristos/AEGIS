-- Audit triggers for critical tables

-- Policy changes
CREATE OR REPLACE FUNCTION audit_policy_changes()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'UPDATE' AND OLD.status != NEW.status) THEN
        INSERT INTO audit_log (
            audit_id,
            org_id,
            user_id,
            action,
            resource_type,
            resource_id,
            changes,
            timestamp
        ) VALUES (
            substr(md5(random()::text || clock_timestamp()::text), 1, 26),
            NEW.org_id,
            COALESCE(current_setting('app.current_user_id', true), 'system'),
            'policy.status_changed',
            'policy',
            NEW.policy_id,
            jsonb_build_object(
                'old_status', OLD.status,
                'new_status', NEW.status,
                'version', NEW.version
            ),
            NOW()
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER policy_audit_trigger
AFTER UPDATE ON policies
FOR EACH ROW
EXECUTE FUNCTION audit_policy_changes();

-- User role changes
CREATE OR REPLACE FUNCTION audit_user_role_changes()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'UPDATE' AND OLD.role != NEW.role) THEN
        INSERT INTO audit_log (
            audit_id,
            org_id,
            user_id,
            action,
            resource_type,
            resource_id,
            changes,
            timestamp
        ) VALUES (
            substr(md5(random()::text || clock_timestamp()::text), 1, 26),
            NEW.org_id,
            COALESCE(current_setting('app.current_user_id', true), 'system'),
            'user.role_changed',
            'user',
            NEW.user_id,
            jsonb_build_object(
                'old_role', OLD.role,
                'new_role', NEW.role,
                'email', NEW.email
            ),
            NOW()
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER user_role_audit_trigger
AFTER UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION audit_user_role_changes();

-- Signing key status changes
CREATE OR REPLACE FUNCTION audit_key_status_changes()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'UPDATE' AND OLD.status != NEW.status) THEN
        INSERT INTO audit_log (
            audit_id,
            org_id,
            user_id,
            action,
            resource_type,
            resource_id,
            changes,
            timestamp
        ) VALUES (
            substr(md5(random()::text || clock_timestamp()::text), 1, 26),
            NEW.org_id,
            COALESCE(current_setting('app.current_user_id', true), 'system'),
            'signing_key.status_changed',
            'signing_key',
            NEW.key_id,
            jsonb_build_object(
                'old_status', OLD.status,
                'new_status', NEW.status
            ),
            NOW()
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER key_status_audit_trigger
AFTER UPDATE ON signing_keys
FOR EACH ROW
EXECUTE FUNCTION audit_key_status_changes();

-- Prevent modification of immutable fields in approved policies
CREATE OR REPLACE FUNCTION prevent_policy_spec_modification()
RETURNS TRIGGER AS $$
BEGIN
    IF (OLD.status IN ('approved', 'deployed') AND 
        OLD.spec::text != NEW.spec::text) THEN
        RAISE EXCEPTION 'Cannot modify spec of approved/deployed policy. Create new version instead.';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER policy_immutability_trigger
BEFORE UPDATE ON policies
FOR EACH ROW
EXECUTE FUNCTION prevent_policy_spec_modification();

-- Prevent event deletion (append-only guarantee)
CREATE OR REPLACE FUNCTION prevent_event_deletion()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Events are append-only and cannot be deleted';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER event_immutability_trigger
BEFORE DELETE ON events
FOR EACH ROW
EXECUTE FUNCTION prevent_event_deletion();

-- Event hash verification trigger
CREATE OR REPLACE FUNCTION verify_event_hash()
RETURNS TRIGGER AS $$
DECLARE
    computed_hash TEXT;
BEGIN
    -- Simple validation: event_hash must be exactly 64 hex chars
    IF NEW.event_hash !~ '^[a-f0-9]{64}$' THEN
        RAISE EXCEPTION 'Invalid event_hash format';
    END IF;
    
    -- Ensure seq_no is monotonic
    IF EXISTS (
        SELECT 1 FROM events 
        WHERE run_id = NEW.run_id AND seq_no >= NEW.seq_no
    ) THEN
        RAISE EXCEPTION 'Event seq_no must be monotonically increasing';
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER event_hash_validation_trigger
BEFORE INSERT ON events
FOR EACH ROW
EXECUTE FUNCTION verify_event_hash();
