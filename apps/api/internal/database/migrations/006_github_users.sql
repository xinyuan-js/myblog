CREATE TABLE IF NOT EXISTS github_users (
    github_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    github_login VARCHAR(100) NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    email VARCHAR(320) NOT NULL,
    avatar_url VARCHAR(2048) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_github_users_login (github_login),
    UNIQUE KEY uq_github_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
