# Changelog

## 1.0.0

- Initial release: packages the `home_assistant_nanit` Go bridge as a
  Home Assistant add-on.
- Auto-wires to the Mosquitto add-on (`services: mqtt:need`).
- Home Assistant MQTT discovery: temperature, humidity, motion, sound, night
  mode and stream sensors plus night-light and standby switches, created
  automatically under a device named after the baby.
- One-time 2FA handled out of band via `get-token.sh`.
