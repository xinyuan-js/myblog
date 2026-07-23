# Keep the sustained rate conservative while allowing normal SPA page loads
# and rapid refreshes to complete without treating them as abuse.
limit_req zone=blog_api_per_ip burst=300 nodelay;
limit_req_status 429;
proxy_pass http://api:8080;
proxy_http_version 1.1;
proxy_set_header Host $host;
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
proxy_connect_timeout 3s;
proxy_send_timeout 30s;
proxy_read_timeout 30s;
# The API sets strict JSON-only headers itself. Hide duplicates and let the
# public gateway apply the site-wide policy exactly once.
proxy_hide_header X-Content-Type-Options;
proxy_hide_header X-Frame-Options;
proxy_hide_header Referrer-Policy;
proxy_hide_header Permissions-Policy;
proxy_hide_header Content-Security-Policy;
