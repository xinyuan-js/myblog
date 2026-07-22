# syntax=docker/dockerfile:1.7
FROM node:22-alpine AS build
WORKDIR /src
COPY apps/web/package.json apps/web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY apps/web/ ./
ARG VITE_API_BASE_URL=/api
ARG VITE_ARTALK_SERVER=/comments
ARG VITE_ARTALK_SITE=MyBlog
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL} \
    VITE_ARTALK_SERVER=${VITE_ARTALK_SERVER} \
    VITE_ARTALK_SITE=${VITE_ARTALK_SITE} \
    VITE_USE_MOCK_API=false
RUN npm run build

FROM nginx:1.31.3-alpine
COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
