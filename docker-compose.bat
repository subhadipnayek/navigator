@echo off
echo Starting Docker Compose...
docker compose up -d --build

if %errorlevel% neq 0 (
echo Failed to start Docker Compose.
) else (
echo Docker Compose started successfully.
)

pause
