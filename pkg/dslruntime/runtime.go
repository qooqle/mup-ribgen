// Package dslruntime provides helper functions used by DSL-compiled
// Dialect Transformer implementations.
package dslruntime

import (
	"encoding/binary"
	"fmt"
	"net"
)

// RT is the zero-value helper struct. Generated code uses it as a namespace.
type RT struct{}

// GetField safely navigates a nested map[string]interface{} using the given keys.
// Returns nil if any intermediate key is missing or has the wrong type.
func (RT) GetField(m interface{}, keys ...string) interface{} {
	cur := m
	for _, k := range keys {
		switch v := cur.(type) {
		case map[string]interface{}:
			next, ok := v[k]
			if !ok {
				return nil
			}
			cur = next
		default:
			return nil
		}
	}
	return cur
}

// GetArray asserts v as []interface{}.
// Returns false if v is nil or not a slice.
func (RT) GetArray(v interface{}) ([]interface{}, bool) {
	arr, ok := v.([]interface{})
	return arr, ok
}

// SetField sets a nested map[string]interface{} key path to val.
// Intermediate maps are created as needed.
func (RT) SetField(m interface{}, val interface{}, keys ...string) {
	mm, ok := m.(map[string]interface{})
	if !ok {
		return
	}
	for i, k := range keys {
		if i == len(keys)-1 {
			mm[k] = val
			return
		}
		next, ok := mm[k]
		if !ok {
			next = map[string]interface{}{}
			mm[k] = next
		}
		mm, ok = next.(map[string]interface{})
		if !ok {
			return
		}
	}
}

// CoerceField returns v unchanged. Used by generated code as a pass-through
// placeholder; callers that need a specific type should use the typed helpers below.
func (RT) CoerceField(v interface{}) interface{} {
	return v
}

// CoerceUint64 converts v to uint64 (hex strings like "0x1" are supported).
func (RT) CoerceUint64(v interface{}) uint64 {
	switch x := v.(type) {
	case uint64:
		return x
	case uint32:
		return uint64(x)
	case uint16:
		return uint64(x)
	case uint8:
		return uint64(x)
	case int:
		return uint64(x)
	case float64:
		return uint64(x)
	case string:
		var n uint64
		fmt.Sscanf(x, "0x%x", &n)
		if n == 0 {
			fmt.Sscanf(x, "%d", &n)
		}
		return n
	}
	return 0
}

// CoerceUint32 converts v to uint32.
func (RT) CoerceUint32(v interface{}) uint32 {
	switch x := v.(type) {
	case uint32:
		return x
	case uint64:
		return uint32(x)
	case uint16:
		return uint32(x)
	case uint8:
		return uint32(x)
	case int:
		return uint32(x)
	case float64:
		return uint32(x)
	case string:
		var n uint32
		fmt.Sscanf(x, "0x%x", &n)
		if n == 0 {
			fmt.Sscanf(x, "%d", &n)
		}
		return n
	}
	return 0
}

// CoerceUint16 converts v to uint16.
func (RT) CoerceUint16(v interface{}) uint16 {
	switch x := v.(type) {
	case uint16:
		return x
	case uint32:
		return uint16(x)
	case uint64:
		return uint16(x)
	case uint8:
		return uint16(x)
	case int:
		return uint16(x)
	case float64:
		return uint16(x)
	case string:
		var n uint16
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

// CoerceUint8 converts v to uint8.
func (RT) CoerceUint8(v interface{}) uint8 {
	switch x := v.(type) {
	case uint8:
		return x
	case uint16:
		return uint8(x)
	case uint32:
		return uint8(x)
	case uint64:
		return uint8(x)
	case int:
		return uint8(x)
	case float64:
		return uint8(x)
	case string:
		var n uint8
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

// CoerceString converts v to string.
func (RT) CoerceString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// --- Built-in transform functions -------------------------------------------

// Transform_network_to_host_u32 converts a 4-byte network-order value to
// a uint32 host-order value.
func (RT) Transform_network_to_host_u32(v interface{}) interface{} {
	switch b := v.(type) {
	case []byte:
		if len(b) == 4 {
			return binary.BigEndian.Uint32(b)
		}
	case uint32:
		return b
	}
	return v
}

// Transform_to_cidr_prefix converts an IP address string to a CIDR prefix
// by appending /prefixLen.
func (RT) Transform_to_cidr_prefix(v interface{}, prefixLen int) interface{} {
	s := fmt.Sprintf("%v", v)
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	return fmt.Sprintf("%s/%d", ip.String(), prefixLen)
}

// Transform_identity returns the value unchanged.
func (RT) Transform_identity(v interface{}) interface{} {
	return v
}
