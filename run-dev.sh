#!/usr/bin/env bash
set -e

# Resolve canonical script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_PATH="${SCRIPT_DIR}/$(basename "${BASH_SOURCE[0]}")"

# Terminal auto-spawn logic for Linux desktop environments
SPAWN_TERMINAL=1
for arg in "$@"; do
    if [ "$arg" == "--in-terminal" ] || [ "$arg" == "--no-spawn" ]; then
        SPAWN_TERMINAL=0
        break
    fi
done

if [ "$SPAWN_TERMINAL" -eq 1 ] && { [ -n "$DISPLAY" ] || [ -n "$WAYLAND_DISPLAY" ]; }; then
    if command -v konsole >/dev/null 2>&1; then
        echo "⚡ Launching navigator orchestrator in Konsole..."
        konsole --workdir "$SCRIPT_DIR" -e bash -c "bash \"$SCRIPT_PATH\" --in-terminal; exec bash" &
        exit 0
    elif command -v gnome-terminal >/dev/null 2>&1; then
        echo "⚡ Launching navigator orchestrator in GNOME Terminal..."
        gnome-terminal --working-directory="$SCRIPT_DIR" -- bash -c "bash \"$SCRIPT_PATH\" --in-terminal; exec bash" &
        exit 0
    elif command -v x-terminal-emulator >/dev/null 2>&1; then
        echo "⚡ Launching navigator orchestrator in Terminal..."
        x-terminal-emulator -e bash -c "cd \"$SCRIPT_DIR\" && bash \"$SCRIPT_PATH\" --in-terminal; exec bash" &
        exit 0
    elif command -v kitty >/dev/null 2>&1; then
        echo "⚡ Launching navigator orchestrator in Kitty..."
        kitty --directory "$SCRIPT_DIR" bash -c "bash \"$SCRIPT_PATH\" --in-terminal; exec bash" &
        exit 0
    elif command -v alacritty >/dev/null 2>&1; then
        echo "⚡ Launching navigator orchestrator in Alacritty..."
        alacritty --working-directory "$SCRIPT_DIR" -e bash -c "bash \"$SCRIPT_PATH\" --in-terminal; exec bash" &
        exit 0
    elif command -v xterm >/dev/null 2>&1; then
        echo "⚡ Launching navigator orchestrator in xterm..."
        xterm -e bash -c "cd \"$SCRIPT_DIR\" && bash \"$SCRIPT_PATH\" --in-terminal; exec bash" &
        exit 0
    fi
fi

# Orchestrator execution loop
cd "$SCRIPT_DIR"

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
WHITE='\033[1;37m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

echo -e "${CYAN}================================================================${NC}"
echo -e "${CYAN}   ⚡ navigator Polyglot Microservices Orchestrator${NC}"
echo -e "${CYAN}================================================================${NC}"
echo ""

echo -e "${YELLOW}[1/3] Gracefully tearing down running containers (docker compose down)...${NC}"
docker compose down || true

echo ""
echo -e "${YELLOW}[2/3] Building and launching microservices in detached mode (docker compose up --build -d)...${NC}"
docker compose up --build -d

echo ""
echo -e "${GREEN}================================================================${NC}"
echo -e "${GREEN}  ✓ [SUCCESS] All navigator Microservices are Live!${NC}"
echo -e "${GREEN}================================================================${NC}"
echo ""
echo -e "${WHITE}  - Angular Cloud IDE:     ${CYAN}http://localhost:4000${NC}"
echo -e "${WHITE}  - Nginx API Edge Proxy:  ${CYAN}http://localhost:5038${NC}"
echo -e "${WHITE}  - Go Live Gateway & PTY: ${CYAN}http://localhost:8081${NC}"
echo -e "${WHITE}  - Go REST Core API:      ${CYAN}http://localhost:5038/api/${NC} ${GRAY}(Internal: 8080)${NC}"
echo -e "${WHITE}  - Node.js Realtime Hub:  ${CYAN}http://localhost:5038/hubs/${NC} ${GRAY}(Internal: 5000)${NC}"
echo -e "${WHITE}  - Python AI Worker:      ${CYAN}http://localhost:5038/api/ai/${NC} ${GRAY}(gRPC: 50051)${NC}"
echo -e "${WHITE}  - PostgreSQL Database:   ${CYAN}localhost:5432${NC}"
echo -e "${WHITE}  - Redis Streams & Bus:   ${CYAN}localhost:6379${NC}"
echo ""
echo -e "${GRAY}Attaching live logs stream (press Ctrl+C to detach without stopping services)...${NC}"
echo -e "${CYAN}================================================================${NC}"
echo ""

# Stream live container logs so user sees continuous progress
docker compose logs -f
