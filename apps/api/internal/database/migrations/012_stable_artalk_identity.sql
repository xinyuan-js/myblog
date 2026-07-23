ALTER TABLE github_users
    ADD COLUMN artalk_user_id BIGINT UNSIGNED NULL AFTER avatar_url,
    DROP INDEX uq_github_users_login,
    DROP INDEX uq_github_users_email,
    ADD KEY idx_github_users_login (github_login),
    ADD KEY idx_github_users_email (email),
    ADD UNIQUE KEY uq_github_users_artalk_user (artalk_user_id);
