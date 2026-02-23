package channel

import "encoding/json"

// Envelope is the wire format for all messages sent over a channel.
// Every message is wrapped in an envelope that carries the type name
// so the receiver can demultiplex by type.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Marshal serializes the envelope to JSON bytes.
func (e Envelope) Marshal() []byte {
	b, err := json.Marshal(e)
	if err != nil {
		// Envelope contains only a string and json.RawMessage, both of which
		// are always serializable. A marshal failure here is a programming error.
		panic("channel.Envelope.Marshal: " + err.Error())
	}
	return b
}

// Unmarshal deserializes JSON bytes into the envelope.
func (e *Envelope) Unmarshal(data []byte) error {
	return json.Unmarshal(data, e)
}
