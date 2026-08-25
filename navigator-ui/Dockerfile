# syntax=docker/dockerfile:1

# Stage 1: Build Angular production bundle with npm cache
FROM node:22-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline --no-audit

COPY . .
RUN npx ng build --configuration production

# Stage 2: High-performance Nginx web server
FROM nginx:1.27-alpine

COPY --from=builder /app/dist/NAVigator/browser /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 4000

HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:4000/ || exit 1

CMD ["nginx", "-g", "daemon off;"]