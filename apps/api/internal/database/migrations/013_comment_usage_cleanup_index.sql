ALTER TABLE comment_daily_usage
    ADD KEY idx_comment_usage_date (usage_date, github_id);
