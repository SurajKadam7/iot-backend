package iot

import (
	"strings"
)

const TelemetryTopicFilter = "org/+/device/+/telemetry"

func ParseTelemetryTopic(topic string) (orgSegment, deviceIdentifier string, ok bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 {
		return "", "", false
	}
	if parts[0] != "org" || parts[2] != "device" || parts[4] != "telemetry" {
		return "", "", false
	}
	if parts[1] == "" || parts[3] == "" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func TelemetryTopic(orgID, deviceIdentifier string) string {
	return "org/" + orgID + "/device/" + deviceIdentifier + "/telemetry"
}
