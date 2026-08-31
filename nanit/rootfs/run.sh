#!/usr/bin/with-contenv bashio
# shellcheck shell=bash
set -e

if ! bashio::services.available "mqtt"; then
  bashio::log.warning "No MQTT service available - install & start the Mosquitto add-on."
fi

export NANIT_LOG_LEVEL="$(bashio::config 'log_level')"
export NANIT_SESSION_FILE="/data/session.json"

if bashio::config.has_value 'nanit_refresh_token'; then
  export NANIT_REFRESH_TOKEN="$(bashio::config 'nanit_refresh_token')"
elif ! bashio::fs.file_exists '/data/session.json'; then
  bashio::exit.nok "No nanit_refresh_token set and no saved session - run get-token.sh and paste the token into the add-on config."
fi

# RTMP camera feed
if bashio::config.has_value 'rtmp_addr'; then
  export NANIT_RTMP_ENABLED="true"
  export NANIT_RTMP_ADDR="$(bashio::config 'rtmp_addr')"
else
  bashio::log.warning "rtmp_addr not set - camera feed disabled, sensors only."
  export NANIT_RTMP_ENABLED="false"
fi

# MQTT - wired automatically to the Mosquitto add-on
export NANIT_MQTT_ENABLED="true"
export NANIT_MQTT_BROKER_URL="mqtt://$(bashio::services mqtt 'host'):$(bashio::services mqtt 'port')"
export NANIT_MQTT_USERNAME="$(bashio::services mqtt 'username')"
export NANIT_MQTT_PASSWORD="$(bashio::services mqtt 'password')"
export NANIT_MQTT_PREFIX="nanit"
export NANIT_MQTT_DISCOVERY="$(bashio::config 'mqtt_discovery')"

# Poll the Nanit event API for motion/sound (websocket alone is not enough)
export NANIT_EVENTS_POLLING="$(bashio::config 'event_polling')"

bashio::log.info "Nanit bridge starting (RTMP='${NANIT_RTMP_ADDR:-off}', MQTT discovery=${NANIT_MQTT_DISCOVERY})"
exec /usr/bin/nanit
