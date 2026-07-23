ALTER TABLE posts
    ADD FULLTEXT KEY ft_posts_search (title, excerpt, content_markdown) WITH PARSER ngram;
