CREATE TABLE IF NOT EXISTS admin_audit_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    actor_github_id BIGINT UNSIGNED NOT NULL,
    actor_login VARCHAR(100) NOT NULL,
    method VARCHAR(10) NOT NULL,
    request_path VARCHAR(500) NOT NULL,
    response_status SMALLINT UNSIGNED NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    client_ip VARCHAR(45) NOT NULL,
    resource_location VARCHAR(500) NOT NULL DEFAULT '',
    occurred_at DATETIME(6) NOT NULL,
    KEY idx_admin_audit_occurred (occurred_at, id),
    KEY idx_admin_audit_actor (actor_github_id, occurred_at, id),
    CONSTRAINT chk_admin_audit_method
        CHECK (method IN ('POST', 'PUT', 'PATCH', 'DELETE')),
    CONSTRAINT chk_admin_audit_status
        CHECK (response_status BETWEEN 100 AND 599)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
