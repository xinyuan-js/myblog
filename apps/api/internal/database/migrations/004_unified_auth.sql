-- Existing administrator-only sessions do not contain the verified GitHub
-- email required by the Artalk identity bridge. Force a one-time re-login and
-- rename the table to reflect that it now stores both readers and admins.
DELETE FROM admin_sessions;
RENAME TABLE admin_sessions TO user_sessions;
ALTER TABLE user_sessions
    ADD COLUMN email VARCHAR(320) NOT NULL AFTER display_name;
