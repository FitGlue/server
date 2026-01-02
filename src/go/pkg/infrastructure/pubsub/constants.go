package pubsub

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// GetCloudEventType returns the string URN for a given CloudEventType enum using the custom ce_type option.
func GetCloudEventType(t pb.CloudEventType) string {
	// Get the Enum Descriptor
	ed := t.Descriptor()
	// Get the specific Enum Value Descriptor
	ev := ed.Values().ByNumber(protoreflect.EnumNumber(t))
	if ev == nil {
		return "unknown"
	}

	// Access options
	opts := ev.Options()

	// Use proto.GetExtension to retrieve the custom option
	// Note: We need the concrete ExtensionType from the generated code (E_CeType)
	if proto.HasExtension(opts, pb.E_CeType) {
		val := proto.GetExtension(opts, pb.E_CeType)
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return "unknown"
}

// GetCloudEventSource returns the string URN for a given CloudEventSource enum using the custom ce_source option.
func GetCloudEventSource(s pb.CloudEventSource) string {
	ed := s.Descriptor()
	ev := ed.Values().ByNumber(protoreflect.EnumNumber(s))
	if ev == nil {
		return "unknown"
	}

	opts := ev.Options()
	if proto.HasExtension(opts, pb.E_CeSource) {
		val := proto.GetExtension(opts, pb.E_CeSource)
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}

	return "unknown"
}
