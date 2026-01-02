package pubsub

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// NewCloudEvent creates a standardized CloudEvent v1.0
func NewCloudEvent(source, eventType string, data interface{}) (cloudevents.Event, error) {
	e := cloudevents.NewEvent()
	e.SetSpecVersion("1.0")
	e.SetType(eventType)
	e.SetSource(source)

	if err := e.SetData(cloudevents.ApplicationJSON, data); err != nil {
		return e, err
	}

	return e, nil
}
