package mqtt

import (
	"fmt"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/indiefan/home_assistant_nanit/pkg/baby"
	"github.com/indiefan/home_assistant_nanit/pkg/utils"
	"github.com/rs/zerolog/log"
)

type SendLightCommandHandler func(nightLightState bool)
type SendStandbyCommandHandler func(standbyState bool)

// Connection - MQTT context
type Connection struct {
	Opts                      Opts
	StateManager              *baby.StateManager
	client                    MQTT.Client
	babies                    map[string]string // uid -> name, for HA discovery
	sendLightCommandHandler   SendLightCommandHandler
	sendStandbyCommandHandler SendStandbyCommandHandler
}

// NewConnection - constructor
func NewConnection(opts Opts) *Connection {
	return &Connection{
		Opts:   opts,
		babies: map[string]string{},
	}
}

// SetBabies - registers uid -> name so HA discovery can name the device.
// Call before Run().
func (conn *Connection) SetBabies(babies []baby.Baby) {
	for _, b := range babies {
		conn.babies[b.UID] = b.Name
	}
}

func (conn *Connection) statusTopic() string {
	return fmt.Sprintf("%v/status", conn.Opts.TopicPrefix)
}

// Run - runs the mqtt connection handler
func (conn *Connection) Run(manager *baby.StateManager, ctx utils.GracefulContext) {
	conn.StateManager = manager

	opts := MQTT.NewClientOptions()
	opts.AddBroker(conn.Opts.BrokerURL)
	opts.SetClientID(conn.Opts.TopicPrefix)
	opts.SetUsername(conn.Opts.Username)
	opts.SetPassword(conn.Opts.Password)
	opts.SetCleanSession(false)
	// Last will so HA marks the entities unavailable if the bridge dies
	opts.SetWill(conn.statusTopic(), "offline", 1, true)

	conn.client = MQTT.NewClient(opts)

	utils.RunWithPerseverance(func(attempt utils.AttemptContext) {
		runMqtt(conn, attempt)
	}, ctx, utils.PerseverenceOpts{
		RunnerID:       "mqtt",
		ResetThreshold: 2 * time.Second,
		Cooldown: []time.Duration{
			2 * time.Second,
			10 * time.Second,
			1 * time.Minute,
		},
	})
}

func (conn *Connection) RegisterLightHandler(sendLightCommandHandler SendLightCommandHandler) {
	conn.sendLightCommandHandler = sendLightCommandHandler
}

func (conn *Connection) subscribeToLightCommand() {
	commandTopic := fmt.Sprintf("%v/babies/+/night_light/switch", conn.Opts.TopicPrefix)
	log.Debug().
		Str("topic", commandTopic).
		Msg("Subscribing to command topic")

	lightMessageHandler := func(mqttConn MQTT.Client, msg MQTT.Message) {
		// Extract baby UID and command from topic
		parts := strings.Split(msg.Topic(), "/")
		if len(parts) < 4 {
			log.Error().Str("topic", msg.Topic()).Msg("Invalid command topic format")
			return
		}

		babyUID := parts[2]
		command := parts[4]

		// Validate baby UID
		baby.EnsureValidBabyUID(babyUID)

		// Handle different commands
		switch command {
		case "switch":
			enabled := string(msg.Payload()) == "true"
			log.Debug().
				Str("baby", babyUID).
				Bool("enabled", enabled).
				Str("payload", string(msg.Payload())).
				Msg("Received light command")

			conn.sendLightCommandHandler(enabled)
		default:
			log.Warn().Str("command", command).Msg("Unknown command received")
		}
	}

	if token := conn.client.Subscribe(commandTopic, 0, lightMessageHandler); token.Wait() && token.Error() != nil {
		log.Error().Err(token.Error()).Str("topic", commandTopic).Msg("Failed to subscribe to command topic")
	}
}

func (conn *Connection) RegisterStandyHandler(sendStandbyCommandHandler SendStandbyCommandHandler) {
	conn.sendStandbyCommandHandler = sendStandbyCommandHandler
}

func (conn *Connection) subscribeToStandbyCommand() {
	commandTopic := fmt.Sprintf("%v/babies/+/standby/switch", conn.Opts.TopicPrefix)
	log.Debug().
		Str("topic", commandTopic).
		Msg("Subscribing to command topic")

	standbyMessageHandler := func(mqttConn MQTT.Client, msg MQTT.Message) {
		// Extract baby UID and command from topic
		parts := strings.Split(msg.Topic(), "/")
		if len(parts) < 4 {
			log.Error().Str("topic", msg.Topic()).Msg("Invalid command topic format")
			return
		}

		babyUID := parts[2]
		command := parts[4]

		// Validate baby UID
		baby.EnsureValidBabyUID(babyUID)

		// Handle different commands
		switch command {
		case "switch":
			enabled := string(msg.Payload()) == "true"
			log.Debug().
				Str("baby", babyUID).
				Bool("enabled", enabled).
				Str("payload", string(msg.Payload())).
				Msg("Received standby command")

			conn.sendStandbyCommandHandler(enabled)
		default:
			log.Warn().Str("command", command).Msg("Unknown command received")
		}
	}

	if token := conn.client.Subscribe(commandTopic, 0, standbyMessageHandler); token.Wait() && token.Error() != nil {
		log.Error().Err(token.Error()).Str("topic", commandTopic).Msg("Failed to subscribe to command topic")
	}
}

func runMqtt(conn *Connection, attempt utils.AttemptContext) {

	if token := conn.client.Connect(); token.Wait() && token.Error() != nil {
		log.Error().Str("broker_url", conn.Opts.BrokerURL).Err(token.Error()).Msg("Unable to connect to MQTT broker")
		attempt.Fail(token.Error())
		return
	}

	log.Info().Str("broker_url", conn.Opts.BrokerURL).Msg("Successfully connected to MQTT broker")

	// Mark the bridge online and (re)publish HA discovery configs
	conn.client.Publish(conn.statusTopic(), 1, true, "online")
	for uid, name := range conn.babies {
		conn.publishDiscovery(uid, name)
	}

	unsubscribe := conn.StateManager.Subscribe(func(babyUID string, state baby.State) {
		publish := func(key string, value interface{}) {
			topic := fmt.Sprintf("%v/babies/%v/%v", conn.Opts.TopicPrefix, babyUID, key)
			log.Trace().Str("topic", topic).Interface("value", value).Msg("MQTT publish")

			token := conn.client.Publish(topic, 0, false, fmt.Sprintf("%v", value))
			if token.Wait(); token.Error() != nil {
				log.Error().Err(token.Error()).Msgf("Unable to publish %v update", key)
			}
		}

		for key, value := range state.AsMap(false) {
			publish(key, value)
		}

		// Derive HA-friendly motion/sound topics from the raw event timestamps:
		//   <key>        -> RFC3339 timestamp (device_class: timestamp)
		//   <key>_active -> "true" for `activeWindow`, then "false"
		if state.MotionTimestamp != nil {
			conn.publishEvent(babyUID, "motion", *state.MotionTimestamp)
		}
		if state.SoundTimestamp != nil {
			conn.publishEvent(babyUID, "sound", *state.SoundTimestamp)
		}

		if state.StreamState != nil && *state.StreamState != baby.StreamState_Unknown {
			publish("is_stream_alive", *state.StreamState == baby.StreamState_Alive)
		}
	})

	// Subscribe to accept light mqtt messages
	conn.subscribeToLightCommand()
	conn.subscribeToStandbyCommand()

	// Wait until interrupt signal is received
	<-attempt.Done()

	log.Debug().Msg("Closing MQTT connection on interrupt")
	unsubscribe()
	conn.client.Publish(conn.statusTopic(), 1, true, "offline")
	conn.client.Disconnect(250)
}

// eventActiveWindow - how long a motion/sound "_active" binary_sensor stays on
// after an event (Nanit only gives us the event timestamp, not a duration).
const eventActiveWindow = 45 * time.Second

// publishEvent - turns a raw motion/sound epoch into HA-friendly topics:
//
//	<key>        retained RFC3339 timestamp (for a device_class: timestamp sensor)
//	<key>_active "true" now, "false" after eventActiveWindow (device_class motion/sound)
func (conn *Connection) publishEvent(babyUID, key string, epoch int32) {
	base := fmt.Sprintf("%v/babies/%v", conn.Opts.TopicPrefix, babyUID)
	ts := time.Unix(int64(epoch), 0).UTC().Format(time.RFC3339)
	conn.client.Publish(fmt.Sprintf("%v/%v", base, key), 0, true, ts)

	activeTopic := fmt.Sprintf("%v/%v_active", base, key)
	conn.client.Publish(activeTopic, 0, false, "true")
	time.AfterFunc(eventActiveWindow, func() {
		conn.client.Publish(activeTopic, 0, false, "false")
	})
}
