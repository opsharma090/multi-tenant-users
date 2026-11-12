BEGIN;

ALTER TABLE tenants
ADD COLUMN permission_version BIGINT NOT NULL DEFAULT 1;

CREATE FUNCTION bump_permission_version()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE tenants
        SET permission_version = permission_version + 1
        WHERE id = OLD.tenant_id;

        RETURN OLD;
    END IF;

    UPDATE tenants
    SET permission_version = permission_version + 1
    WHERE id = NEW.tenant_id;

    IF TG_OP = 'UPDATE' AND OLD.tenant_id <> NEW.tenant_id THEN
        UPDATE tenants
        SET permission_version = permission_version + 1
        WHERE id = OLD.tenant_id;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER user_roles_permission_version
AFTER INSERT OR UPDATE OR DELETE ON user_roles
FOR EACH ROW EXECUTE FUNCTION bump_permission_version();

CREATE TRIGGER role_permissions_permission_version
AFTER INSERT OR UPDATE OR DELETE ON role_permissions
FOR EACH ROW EXECUTE FUNCTION bump_permission_version();

COMMIT;