package mqtt

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
)

// Home Assistant MQTT discovery.
//
// On (re)connect we publish a retained discovery config for every entity of
// every known baby to `<discovery_prefix>/<component>/nanit_<uid>/<object>/config`.
// Home Assistant then creates a "Nanit" device with all of the sensors and the
// two writable controls (night light, standby) automatically - no YAML.

type haDevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
}

type haAvailability struct {
	Topic               string `json:"topic"`
	PayloadAvailable    string `json:"payload_available"`
	PayloadNotAvailable string `json:"payload_not_available"`
}

type haEntity struct {
	Name              string           `json:"name"`
	UniqueID          string           `json:"unique_id"`
	StateTopic        string           `json:"state_topic"`
	CommandTopic      string           `json:"command_topic,omitempty"`
	DeviceClass       string           `json:"device_class,omitempty"`
	StateClass        string           `json:"state_class,omitempty"`
	UnitOfMeasurement string           `json:"unit_of_measurement,omitempty"`
	Icon              string           `json:"icon,omitempty"`
	PayloadOn         string           `json:"payload_on,omitempty"`
	PayloadOff        string           `json:"payload_off,omitempty"`
	StateOn           string           `json:"state_on,omitempty"`
	StateOff          string           `json:"state_off,omitempty"`
	Availability      []haAvailability `json:"availability,omitempty"`
	Device            haDevice         `json:"device"`
}

type entitySpec struct {
	component string // "sensor" | "binary_sensor" | "switch"
	object    string // object_id suffix
	friendly  string
	topic     string // state topic key (under nanit/babies/<uid>/)
	cmd       string // command topic key, for switches
	class     string
	stateCls  string
	unit      string
	icon      string
}

var entitySpecs = []entitySpec{
	{"sensor", "temperature", "Temperature", "temperature", "", "temperature", "measurement", "°C", ""},
	{"sensor", "humidity", "Humidity", "humidity", "", "humidity", "measurement", "%", ""},
	{"sensor", "motion", "Last motion", "motion", "", "timestamp", "", "", "mdi:motion-sensor"},
	{"sensor", "sound", "Last sound", "sound", "", "timestamp", "", "", "mdi:ear-hearing"},
	{"binary_sensor", "motion_active", "Motion", "motion_active", "", "motion", "", "", ""},
	{"binary_sensor", "sound_active", "Sound", "sound_active", "", "sound", "", "", ""},
	{"binary_sensor", "night", "Night mode", "is_night", "", "", "", "", "mdi:weather-night"},
	{"binary_sensor", "stream", "Stream", "is_stream_alive", "", "connectivity", "", "", ""},
	{"switch", "night_light", "Night light", "night_light", "night_light/switch", "", "", "", "mdi:lightbulb-night"},
	{"switch", "standby", "Standby", "standby", "standby/switch", "", "", "", "mdi:power-standby"},
}

// publishDiscovery - publishes retained HA discovery configs for one baby
func (conn *Connection) publishDiscovery(babyUID, babyName string) {
	if !conn.Opts.DiscoveryEnabled {
		return
	}

	discoveryPrefix := conn.Opts.DiscoveryPrefix
	if discoveryPrefix == "" {
		discoveryPrefix = "homeassistant"
	}

	name := babyName
	if name == "" {
		short := babyUID
		if len(short) > 6 {
			short = short[:6]
		}
		name = "Nanit " + short
	}

	dev := haDevice{
		Identifiers:  []string{"nanit_" + babyUID},
		Name:         name,
		Manufacturer: "Nanit",
		Model:        "Baby Monitor",
	}
	avail := []haAvailability{{
		Topic:               fmt.Sprintf("%v/status", conn.Opts.TopicPrefix),
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
	}}

	base := fmt.Sprintf("%v/babies/%v", conn.Opts.TopicPrefix, babyUID)

	for _, s := range entitySpecs {
		e := haEntity{
			Name:              s.friendly,
			UniqueID:          fmt.Sprintf("nanit_%v_%v", babyUID, s.object),
			StateTopic:        fmt.Sprintf("%v/%v", base, s.topic),
			DeviceClass:       s.class,
			StateClass:        s.stateCls,
			UnitOfMeasurement: s.unit,
			Icon:              s.icon,
			Availability:      avail,
			Device:            dev,
		}

		if s.component == "binary_sensor" || s.component == "switch" {
			e.PayloadOn = "true"
			e.PayloadOff = "false"
		}
		if s.component == "switch" {
			e.CommandTopic = fmt.Sprintf("%v/%v", base, s.cmd)
			e.StateOn = "true"
			e.StateOff = "false"
		}

		payload, err := json.Marshal(e)
		if err != nil {
			log.Error().Err(err).Str("object", s.object).Msg("Unable to marshal discovery config")
			continue
		}

		topic := fmt.Sprintf("%v/%v/nanit_%v/%v/config", discoveryPrefix, s.component, babyUID, s.object)
		if token := conn.client.Publish(topic, 1, true, payload); token.Wait() && token.Error() != nil {
			log.Error().Err(token.Error()).Str("topic", topic).Msg("Unable to publish discovery config")
		} else {
			log.Debug().Str("topic", topic).Msg("Published HA discovery config")
		}
	}

	log.Info().Str("baby", babyUID).Str("name", name).Msg("Published Home Assistant MQTT discovery")
}
