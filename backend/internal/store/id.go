package store

import (
	uuid "github.com/hashicorp/go-uuid"
)

// NewID returns a random, unique identifier for a new User or Group
// record. Uses hashicorp/go-uuid since it's already pulled in via
// hashicorp/raft.
func NewID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic("store: failed to generate random id: " + err.Error())
	}
	return id
}
