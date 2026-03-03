package server

import (
	"io"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var protoUnmarshaler = protojson.UnmarshalOptions{DiscardUnknown: true}

// decodeProto reads the request body and unmarshals it into a protobuf message
// using protojson, which correctly handles enum string names, structpb.Struct, etc.
func decodeProto(r *http.Request, msg proto.Message) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return protoUnmarshaler.Unmarshal(body, msg)
}
