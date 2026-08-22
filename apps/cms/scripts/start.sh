#!/usr/bin/env sh
set -eu

npm run migrate
exec node .next/standalone/server.js
