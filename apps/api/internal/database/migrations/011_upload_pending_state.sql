ALTER TABLE uploads
    DROP CHECK chk_uploads_status,
    ADD CONSTRAINT chk_uploads_status
        CHECK (status IN ('active', 'trashed', 'deleting', 'uploading'));
