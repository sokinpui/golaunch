gol add ssh-tunnel-jump \
  --env SSH_AUTH_SOCK="$SSH_AUTH_SOCK" \
  "ssh -N -o BatchMode=yes -o ControlMaster=no -o ControlPath=none -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes -R 0.0.0.0:2352:localhost:2352 -R 0.0.0.0:9001:localhost:9001 -D 0.0.0.0:9999 jump"

gol add ssh-tunnel-v100 \
  --env SSH_AUTH_SOCK="$SSH_AUTH_SOCK" \
  "ssh -N -o BatchMode=yes -o ControlMaster=no -o ControlPath=none -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes  -L 0.0.0.0:10000:localhost:10000 v100"

./gol add synapse --cwd /Users/so/projects/synapse/main ./start.sh

./gol add ai-studio-to-api \
  --cwd /Users/so/projects/AIStudioToAPI \
  --env http_proxy=http://127.0.0.1:1087 \
  --env https_proxy=http://127.0.0.1:1087 \
  --env ALL_PROXY=socks5://127.0.0.1:1080 \
  npm start

gol add coder-server --cwd /Users/so/projects/coder.server "coder --ws --port 9005"
