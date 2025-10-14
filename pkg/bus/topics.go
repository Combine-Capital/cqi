package bus

import (
	"fmt"
	"strings"
)

const (
	// topicPrefix is the standard prefix for all CQC event topics.
	topicPrefix = "cqc.events.v1"
)

// TopicName generates a topic name following the CQC naming convention.
// The event type should be in snake_case (e.g., "asset_created", "position_changed").
// Returns a topic name in the format "cqc.events.v1.{event_type}".
//
// Example:
//
//	topic := bus.TopicName("asset_created")
//	// Returns: "cqc.events.v1.asset_created"
func TopicName(eventType string) string {
	// Ensure event type is lowercase and use snake_case
	eventType = strings.ToLower(eventType)
	return fmt.Sprintf("%s.%s", topicPrefix, eventType)
}

// ParseEventType extracts the event type from a topic name.
// Returns the event type if the topic follows the CQC naming convention,
// or the original topic if it doesn't match the expected format.
//
// Example:
//
//	eventType := bus.ParseEventType("cqc.events.v1.asset_created")
//	// Returns: "asset_created"
func ParseEventType(topic string) string {
	prefix := topicPrefix + "."
	if strings.HasPrefix(topic, prefix) {
		return strings.TrimPrefix(topic, prefix)
	}
	return topic
}

// IsValidTopic checks if a topic name follows the CQC naming convention.
// Returns true if the topic starts with "cqc.events.v1." and has an event type.
func IsValidTopic(topic string) bool {
	prefix := topicPrefix + "."
	return strings.HasPrefix(topic, prefix) && len(topic) > len(prefix)
}
