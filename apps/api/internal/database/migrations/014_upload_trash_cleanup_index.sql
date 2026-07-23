ALTER TABLE uploads
    ADD KEY idx_uploads_status_trashed (status, trashed_at, id);
