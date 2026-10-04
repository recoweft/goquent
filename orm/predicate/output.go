package predicate

import (
	"encoding/json"

	"github.com/recoweft/goquent/orm/internal/valueguard"
)

// MarshalJSON applies the same built-in output limits to standalone condition
// views as to plans. Unverified generated payloads remain omitted by the builder.
func (n Node) MarshalJSON() ([]byte, error) {
	type plain Node
	if err := valueguard.Check(plain(n)); err != nil {
		return nil, err
	}
	return json.Marshal(plain(n))
}
func (v Value) MarshalJSON() ([]byte, error) {
	type plain Value
	if err := valueguard.Check(plain(v)); err != nil {
		return nil, err
	}
	return json.Marshal(plain(v))
}
