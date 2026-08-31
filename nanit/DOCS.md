# Nanit Bridge

Runs the [`home_assistant_nanit`](https://github.com/WColan/home_assistant_nanit)
Go bridge as a Home Assistant add-on: a local **RTMP camera feed** plus **MQTT
sensors and controls** for your Nanit baby monitor, created automatically in
Home Assistant via MQTT discovery.

## Entities created

A device named after your baby, with:

| Entity | Type | Notes |
|---|---|---|
| Temperature | sensor | °C from the camera's sensor |
| Humidity | sensor | % RH |
| Last motion / Last sound | sensor | timestamp of the last event |
| Motion / Sound | binary_sensor | on for ~45 s after an event |
| Night mode | binary_sensor | camera day/night state |
| Stream | binary_sensor | RTMP feed health |
| Night light | switch | turns the camera's night light on/off |
| Standby | switch | puts the camera in standby |

The camera itself is added separately as an `ffmpeg` camera pointing at
`rtmp://<rtmp_addr>/local/<baby_uid>` (see below).

## Setup

### 1. Install the Mosquitto broker add-on

This add-on wires itself to Mosquitto automatically — no MQTT config needed here.

### 2. Get a Nanit refresh token

Nanit requires 2FA, so authentication happens once, outside the add-on. On any
machine with `bash`, `curl` and `jq` (your Mac is fine):

```bash
curl -sSL https://raw.githubusercontent.com/WColan/home_assistant_nanit/main/nanit/get-token.sh | bash
```

Enter your Nanit email/password, then the code it emails you. It prints a
**refresh token**. Copy it.

> The refresh token grants full access to your Nanit account. Treat it like a
> password.

### 3. Configure the add-on

| Option | Value |
|---|---|
| `nanit_refresh_token` | the token from step 2 |
| `rtmp_addr` | `<home-assistant-ip>:1935` — an address the **camera** can reach (usually your HA box's LAN IP). Leave blank for sensors only. |
| `mqtt_discovery` | `true` |
| `event_polling` | `true` (needed for motion/sound) |
| `log_level` | `info` |

Start the add-on. On first run, check the log for the line reporting your
`baby_uid` — you'll need it for the camera.

### 4. Add the camera (optional)

In `configuration.yaml`:

```yaml
camera:
  - platform: ffmpeg
    name: Nanit
    input: "rtmp://<rtmp_addr>/local/<baby_uid>"
```

## Notes

- The camera has one local RTMP slot. If the Nanit app is streaming locally at
  the same time, one of the two will drop.
- If your HA box is on Wi-Fi a hop away from the camera and the RTMP feed won't
  hold, run the RTMP part on a box on the same LAN as the camera and leave
  `rtmp_addr` blank here (MQTT still works).
- Sensor updates are pushed by the camera every few minutes and on change.
