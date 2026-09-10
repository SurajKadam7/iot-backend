package iot

import "testing"

func TestParseTelemetryTopic(t *testing.T) {
	org, id, ok := ParseTelemetryTopic("org/abc/device/dev-1/telemetry")
	if !ok || org != "abc" || id != "dev-1" {
		t.Fatalf("%s %s %v", org, id, ok)
	}
	if _, _, ok := ParseTelemetryTopic("org/abc/device/dev-1"); ok {
		t.Fatal("short topic")
	}
	if _, _, ok := ParseTelemetryTopic("x/abc/device/dev-1/telemetry"); ok {
		t.Fatal("bad prefix")
	}
}
