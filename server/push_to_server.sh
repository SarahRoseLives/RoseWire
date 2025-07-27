#!/bin/bash

# Variables
LOCAL_BINARY="rosewire-server"
REMOTE_USER="rose"
REMOTE_HOST="sarahsforge.dev"
REMOTE_PATH="/home/rose/rosewire-server"

PASSWORD="Srl0971304404741050!"

# 1. Copy rosewire-server to the remote server
echo "Copying $LOCAL_BINARY to $REMOTE_USER@$REMOTE_HOST:$REMOTE_PATH ..."

if [ -n "$PASSWORD" ]; then
  # Requires sshpass to be installed
  sshpass -p "$PASSWORD" scp "$LOCAL_BINARY" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_PATH"
else
  scp "$LOCAL_BINARY" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_PATH"
fi

if [ $? -ne 0 ]; then
  echo "SCP failed. Exiting."
  exit 1
fi

# 2. SSH into remote host, cd to /home/rose, and start rosewire-server
echo "Connecting to $REMOTE_USER@$REMOTE_HOST ..."

if [ -n "$PASSWORD" ]; then
  sshpass -p "$PASSWORD" ssh "$REMOTE_USER@$REMOTE_HOST" <<'ENDSSH'
cd /home/rose
echo "Running rosewire-server (Ctrl+C to stop, or kill process to stop remotely)..."
chmod +x ./rosewire-server
./rosewire-server
ENDSSH
else
  ssh "$REMOTE_USER@$REMOTE_HOST" <<'ENDSSH'
cd /home/rose
echo "Running rosewire-server (Ctrl+C to stop, or kill process to stop remotely)..."
chmod +x ./rosewire-server
./rosewire-server
ENDSSH
fi
