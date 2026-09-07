package pubsub

import (
	"encoding/json"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// NewCloudEvent creates a standardized CloudEvent v1.0
func NewCloudEvent(source, eventType string, data interface{}) (cloudevents.Event, error) {
	e := cloudevents.NewEvent()
	e.SetSpecVersion("1.0")
	e.SetType(eventType)
	e.SetSource(source)
	// Stamp creation time: the enricher's lag-exhaustion logic ages events via
	// e.Time(), which stays zero unless explicitly set (the SDK doesn't default it).
	e.SetTime(time.Now())

	// If data is a protobuf message, use protojson to ensure correct JSON formatting (e.g. timestamps as strings)
	if msg, ok := data.(proto.Message); ok {
		opts := protojson.MarshalOptions{
			UseProtoNames: true,
		}
		bytes, err := opts.Marshal(msg)
		if err != nil {
			return e, err
		}
		// Wrap in json.RawMessage so it's not base64 encoded
		if err := e.SetData(cloudevents.ApplicationJSON, json.RawMessage(bytes)); err != nil {
			return e, err
		}
	} else {
		// Fallback to standard JSON for non-proto types
		if err := e.SetData(cloudevents.ApplicationJSON, data); err != nil {
			return e, err
		}
	}

	return e, nil
}
