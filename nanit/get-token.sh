#!/usr/bin/env bash
# Run this ONCE on any machine with bash + curl + jq (e.g. your Mac) to get a
# Nanit refresh token. Paste the printed token into the add-on's configuration.
# The token is long-lived and refreshes itself; you only redo this if it's revoked.
set -euo pipefail

command -v jq >/dev/null || { echo "install jq first (brew install jq)"; exit 1; }

read -rp 'Nanit email: ' EMAIL
read -rsp 'Nanit password: ' PASSWORD; echo

MFA=$(curl -sS -H 'nanit-api-version: 1' -H 'Content-Type: application/json' \
  -d "$(jq -n --arg e "$EMAIL" --arg p "$PASSWORD" '{email:$e,password:$p,channel:"email"}')" \
  https://api.nanit.com/login | jq -r '.mfa_token // empty')

[ -n "$MFA" ] || { echo "Login failed - check email/password."; exit 1; }
echo "A 2FA code was emailed to you."
read -rp 'Code (include quotes if it has leading zeros, e.g. "0042"): ' CODE

TOKEN=$(curl -sS -H 'nanit-api-version: 1' -H 'Content-Type: application/json' \
  -d "$(jq -n --arg e "$EMAIL" --arg p "$PASSWORD" --arg t "$MFA" --argjson c "$CODE" \
        '{email:$e,password:$p,mfa_token:$t,mfa_code:$c,channel:"email"}')" \
  https://api.nanit.com/login | jq -r '.refresh_token // empty')

[ -n "$TOKEN" ] || { echo "2FA failed - wrong code?"; exit 1; }
echo
echo "Refresh token (paste into the add-on config as nanit_refresh_token):"
echo
echo "$TOKEN"
