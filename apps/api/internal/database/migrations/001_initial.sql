CREATE TABLE IF NOT EXISTS site_settings (
    id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
    title VARCHAR(120) NOT NULL,
    subtitle VARCHAR(200) NOT NULL,
    description VARCHAR(500) NOT NULL,
    avatar_url VARCHAR(2048) NULL,
    banner_url VARCHAR(2048) NULL,
    author_name VARCHAR(120) NOT NULL,
    author_bio VARCHAR(500) NOT NULL,
    social_links JSON NOT NULL,
    icp_number VARCHAR(100) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    CONSTRAINT chk_site_settings_singleton CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO site_settings (
    id, title, subtitle, description, author_name, author_bio, social_links
) VALUES (
    1, 'MyBlog', '记录、思考与分享', '一个使用 Go 和 Vue 构建的个人博客。',
    '博主', '欢迎来到我的博客。', JSON_ARRAY()
) ON DUPLICATE KEY UPDATE id = VALUES(id);

CREATE TABLE IF NOT EXISTS categories (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    slug VARCHAR(80) NOT NULL,
    description VARCHAR(300) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_categories_name (name),
    UNIQUE KEY uq_categories_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tags (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    slug VARCHAR(80) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_tags_name (name),
    UNIQUE KEY uq_tags_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS posts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    slug VARCHAR(160) NOT NULL,
    excerpt VARCHAR(500) NOT NULL,
    content_markdown MEDIUMTEXT NOT NULL,
    cover_url VARCHAR(2048) NULL,
    category_id BIGINT UNSIGNED NULL,
    status VARCHAR(20) NOT NULL,
    published_at DATETIME(6) NULL,
    word_count INT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    UNIQUE KEY uq_posts_slug (slug),
    KEY idx_posts_public (status, published_at, id),
    KEY idx_posts_category (category_id),
    KEY idx_posts_deleted (deleted_at),
    CONSTRAINT fk_posts_category FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT,
    CONSTRAINT chk_posts_status CHECK (status IN ('draft', 'published'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS post_tags (
    post_id BIGINT UNSIGNED NOT NULL,
    tag_id BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (post_id, tag_id),
    KEY idx_post_tags_tag (tag_id, post_id),
    CONSTRAINT fk_post_tags_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    CONSTRAINT fk_post_tags_tag FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS admin_sessions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    token_hash BINARY(32) NOT NULL,
    csrf_token_hash BINARY(32) NOT NULL,
    github_id BIGINT UNSIGNED NOT NULL,
    github_login VARCHAR(100) NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    avatar_url VARCHAR(2048) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    last_seen_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_admin_sessions_token (token_hash),
    KEY idx_admin_sessions_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS oauth_states (
    nonce_hash BINARY(32) NOT NULL PRIMARY KEY,
    expires_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY idx_oauth_states_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS uploads (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    storage_key VARCHAR(512) NOT NULL,
    public_url VARCHAR(2048) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size BIGINT UNSIGNED NOT NULL,
    width INT UNSIGNED NOT NULL,
    height INT UNSIGNED NOT NULL,
    sha256 BINARY(32) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    trashed_at DATETIME(6) NULL,
    UNIQUE KEY uq_uploads_storage_key (storage_key),
    KEY idx_uploads_status_created (status, created_at, id),
    KEY idx_uploads_sha256 (sha256),
    CONSTRAINT chk_uploads_status CHECK (status IN ('active', 'trashed', 'deleting'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS upload_references (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    upload_id BIGINT UNSIGNED NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    resource_id BIGINT UNSIGNED NULL,
    field_name VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY idx_upload_references_upload (upload_id),
    KEY idx_upload_references_resource (resource_type, resource_id),
    UNIQUE KEY uq_upload_reference (upload_id, resource_type, resource_id, field_name),
    CONSTRAINT fk_upload_references_upload FOREIGN KEY (upload_id) REFERENCES uploads(id) ON DELETE CASCADE,
    CONSTRAINT chk_upload_reference_type CHECK (resource_type IN ('site_avatar', 'site_banner', 'post_cover', 'post_content'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
