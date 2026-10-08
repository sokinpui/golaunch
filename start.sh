#!/bin/bash
set -e

# Build gol binary
go build -o gol ./cmd/gol/

# 1. synapse
./gol add synapse --cwd /Users/so/projects/synapse/main "./start.sh"

# 2. coder-server
./gol add coder-server --cwd /Users/so/projects/coder.server "coder --ws --port 9005"

# 3. ai-studio-to-api
./gol add ai-studio-to-api \
  --cwd /Users/so/projects/AIStudioToAPI \
  --env http_proxy=http://127.0.0.1:1087 \
  --env https_proxy=http://127.0.0.1:1087 \
  --env ALL_PROXY=socks5://127.0.0.1:1080 \
  "npm start"

# 5. ssh-tunnel2
./gol add ssh-tunnel2 "ssh -N -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes -R 0.0.0.0:2352:localhost:2352 -R 0.0.0.0:9001:localhost:9001 -D 0.0.0.0:9999 jump"

# 6. ssh-tunnel3
./gol add ssh-tunnel3 "ssh -N -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes -L 0.0.0.0:10000:localhost:10000 v100"

echo "Migration complete!"
./gol ls
