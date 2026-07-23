ALTER TABLE github_users
    ADD COLUMN comments_blocked BOOLEAN NOT NULL DEFAULT FALSE AFTER avatar_url,
    ADD COLUMN comment_block_reason VARCHAR(500) NOT NULL DEFAULT '' AFTER comments_blocked,
    ADD COLUMN comment_daily_limit INT UNSIGNED NULL AFTER comment_block_reason;

CREATE TABLE IF NOT EXISTS comment_daily_usage (
    github_id BIGINT UNSIGNED NOT NULL,
    usage_date DATE NOT NULL,
    comment_count INT UNSIGNED NOT NULL DEFAULT 0,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (github_id, usage_date),
    CONSTRAINT fk_comment_daily_usage_user
        FOREIGN KEY (github_id) REFERENCES github_users(github_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
