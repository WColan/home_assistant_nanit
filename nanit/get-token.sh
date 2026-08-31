#!/usr/bin/env bash
# Run this ONCE on any machine with bash + curl + jq (e.g. your Mac) to get a
# Nanit refresh token. Paste the printed token into the add-on's configuration.
# The token is long-lived and refreshes itself; you only redo this if it's revoked.
#
#   curl -sSL https://raw.githubusercontent.com/WColan/home_assistant_nanit/main/nanit/get-token.sh | bash
#
set -euo pipefail

command -v jq >/dev/null || { echo "install jq first (brew install jq)"; exit 1; }

# Read prompts from the terminal, not stdin - so `curl ... | bash` still works.
if [ -r /dev/tty ]; then exec 3</dev/tty; else exec 3<&0; fi

printf 'Nanit email: ' >&2;    IFS= read -r EMAIL <&3
printf 'Nanit password: ' >&2; IFS= read -rs PASSWORD <&3; echo >&2

MFA=$(curl -sS -H 'nanit-api-version: 1' -H 'Content-Type: application/json' \
  -d "$(jq -n --arg e "$EMAIL" --arg p "$PASSWORD" '{email:$e,password:$p,channel:"email"}')" \
  https://api.nanit.com/login | jq -r '.mfa_token // empty')

[ -n "$MFA" ] || { echo "Login failed - check email/password." >&2; exit 1; }
echo "A 2FA code was emailed to you." >&2
printf 'Code from the email: ' >&2
IFS= read -r CODE <&3
CODE="${CODE//\"/}"   # tolerate pasted quotes

TOKEN=$(curl -sS -H 'nanit-api-version: 1' -H 'Content-Type: application/json' \
  -d "$(jq -n --arg e "$EMAIL" --arg p "$PASSWORD" --arg t "$MFA" --arg c "$CODE" \
        '{email:$e,password:$p,mfa_token:$t,mfa_code:$c,channel:"email"}')" \
  https://api.nanit.com/login | jq -r '.refresh_token // empty')

[ -n "$TOKEN" ] || { echo "2FA failed - wrong code?" >&2; exit 1; }
echo >&2
echo "Refresh token (paste into the add-on config as nanit_refresh_token):" >&2
echo >&2
echo "$TOKEN"
