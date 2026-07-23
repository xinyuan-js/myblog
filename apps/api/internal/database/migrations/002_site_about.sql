ALTER TABLE site_settings
    ADD COLUMN about_markdown MEDIUMTEXT NULL AFTER author_bio;

UPDATE site_settings
SET about_markdown = CONCAT(
    '# 你好，我是', author_name, '。', '\n\n',
    author_bio, '\n\n',
    '## 关于这个博客', '\n\n',
    description
)
WHERE about_markdown IS NULL;

ALTER TABLE site_settings
    MODIFY about_markdown MEDIUMTEXT NOT NULL;

ALTER TABLE upload_references
    DROP CHECK chk_upload_reference_type,
    ADD CONSTRAINT chk_upload_reference_type
        CHECK (resource_type IN ('site_avatar', 'site_banner', 'site_about', 'post_cover', 'post_content'));
