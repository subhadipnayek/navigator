# Stage 1 — Build
FROM node:24-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npx ng build --configuration production
 
# Stage 2 — Serve
FROM nginx:alpine
COPY --from=builder /app/dist/NAVigator/browser /usr/share/nginx/html
EXPOSE 4000
COPY nginx.conf /etc/nginx/conf.d/default.conf