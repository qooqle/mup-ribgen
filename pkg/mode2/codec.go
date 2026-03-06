package mode2

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

// CodecName is the gRPC content subtype used for the JSON codec.
// Clients must pass grpc.CallContentSubtype(CodecName) to use this codec.
const CodecName = "json"

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                               { return CodecName }
