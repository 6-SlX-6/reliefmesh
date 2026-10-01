package httpx

import (
	"bytes"
	"encoding/json"
)

// Optional distinguishes an absent JSON field from an explicit null in
// PATCH payloads.
type Optional[T any] struct {
	Set   bool // the field was present
	Valid bool // the field was present and not null
	V     T
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Valid = false
		var zero T
		o.V = zero
		return nil
	}
	if err := json.Unmarshal(b, &o.V); err != nil {
		return err
	}
	o.Valid = true
	return nil
}

// Ptr returns a pointer to the value, or nil when null.
func (o Optional[T]) Ptr() *T {
	if !o.Valid {
		return nil
	}
	v := o.V
	return &v
}
