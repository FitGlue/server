package types

// PubSubMessage is the payload of a Pub/Sub event via Cloud Event.
type PubSubMessage struct {
	Message struct {
		Data []byte `json:"data"`
	} `json:"message"`
}
