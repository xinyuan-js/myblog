INSERT INTO github_users (github_id, github_login, display_name, email, avatar_url)
SELECT session.github_id, session.github_login, session.display_name, session.email, session.avatar_url
FROM user_sessions AS session
INNER JOIN (
    SELECT github_id, MAX(id) AS id
    FROM user_sessions
    GROUP BY github_id
) AS latest ON latest.id = session.id
ON DUPLICATE KEY UPDATE
    github_login = VALUES(github_login),
    display_name = VALUES(display_name),
    email = VALUES(email),
    avatar_url = VALUES(avatar_url);
