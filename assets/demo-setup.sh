#!/bin/bash
# Setup script for agent-mongo demo recording.
#
# Safety, because this tool talks to real databases:
#   * The data lives in a throwaway Docker container, on port 27018 so it
#     cannot collide with a real local mongod on 27017. It is destroyed and
#     recreated on every run.
#   * XDG_CONFIG_HOME is redirected to /tmp, so the demo writes its own
#     config.json and never reads or edits ~/.config/agent-mongo/.
#   * AGENT_MONGO_NO_KEYCHAIN=1 makes the keyring report unavailable, so
#     nothing is written to the OS keychain.
#   * The connection has no credential at all — it is an unauthenticated
#     localhost mongod holding invented data.
set -e

CONTAINER=agent-mongo-demo
PORT=27018

docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER" -p "$PORT:27017" mongo:8 >/dev/null

# Wait for it to accept connections.
for _ in $(seq 1 60); do
  if docker exec "$CONTAINER" mongosh --quiet --eval 'db.runCommand({ping:1}).ok' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker exec "$CONTAINER" mongosh --quiet shop --eval '
db.users.insertMany([
  { email: "ada@example.com",    name: "Ada Lovelace",   active: true,  age: 36, plan: "pro",  createdAt: new Date("2026-01-14") },
  { email: "grace@example.com",  name: "Grace Hopper",   active: true,  age: 45, plan: "pro",  createdAt: new Date("2026-02-02") },
  { email: "alan@example.com",   name: "Alan Turing",    active: false, age: 41, plan: "free", createdAt: new Date("2026-02-19") },
  { email: "katherine@example.com", name: "Katherine Johnson", active: true, age: 52, plan: "team", createdAt: new Date("2026-03-05") }
]);
db.orders.insertMany([
  { ref: "SO-1001", status: "paid",    total: 4200, currency: "GBP", items: 3 },
  { ref: "SO-1002", status: "pending", total: 1150, currency: "GBP", items: 1 },
  { ref: "SO-1003", status: "paid",    total: 8990, currency: "USD", items: 5 },
  { ref: "SO-1004", status: "pending", total:  450, currency: "EUR", items: 1 },
  { ref: "SO-1005", status: "refunded",total: 2300, currency: "GBP", items: 2 }
]);
db.users.createIndex({ email: 1 }, { unique: true });
db.orders.createIndex({ status: 1 });
' >/dev/null

# Isolated config — never ~/.config/agent-mongo.
rm -rf /tmp/agent-mongo-demo
mkdir -p /tmp/agent-mongo-demo/config
export XDG_CONFIG_HOME=/tmp/agent-mongo-demo/config
export AGENT_MONGO_NO_KEYCHAIN=1

agent-mongo connection add demo "mongodb://127.0.0.1:$PORT/shop" --default >/dev/null
