package bus

import "testing"

func TestTopicName(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		want      string
	}{
		{
			name:      "simple event type",
			eventType: "asset_created",
			want:      "cqc.events.v1.asset_created",
		},
		{
			name:      "uppercase converted",
			eventType: "POSITION_CHANGED",
			want:      "cqc.events.v1.position_changed",
		},
		{
			name:      "mixed case converted",
			eventType: "PriceUpdated",
			want:      "cqc.events.v1.priceupdated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopicName(tt.eventType)
			if got != tt.want {
				t.Errorf("TopicName(%q) = %q, want %q", tt.eventType, got, tt.want)
			}
		})
	}
}

func TestParseEventType(t *testing.T) {
	tests := []struct {
		name  string
		topic string
		want  string
	}{
		{
			name:  "valid cqc topic",
			topic: "cqc.events.v1.asset_created",
			want:  "asset_created",
		},
		{
			name:  "invalid topic returns original",
			topic: "other.topic",
			want:  "other.topic",
		},
		{
			name:  "empty event type",
			topic: "cqc.events.v1.",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEventType(tt.topic)
			if got != tt.want {
				t.Errorf("ParseEventType(%q) = %q, want %q", tt.topic, got, tt.want)
			}
		})
	}
}

func TestIsValidTopic(t *testing.T) {
	tests := []struct {
		name  string
		topic string
		want  bool
	}{
		{
			name:  "valid topic",
			topic: "cqc.events.v1.asset_created",
			want:  true,
		},
		{
			name:  "invalid prefix",
			topic: "other.events.v1.test",
			want:  false,
		},
		{
			name:  "missing event type",
			topic: "cqc.events.v1.",
			want:  false,
		},
		{
			name:  "empty string",
			topic: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidTopic(tt.topic)
			if got != tt.want {
				t.Errorf("IsValidTopic(%q) = %v, want %v", tt.topic, got, tt.want)
			}
		})
	}
}
