package mongo

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"time"
)

const (
	defaultTimeout       = 10 * time.Minute
	defaultCheckInterval = 5 * time.Second
	highPriority         = 1
	lowPriority          = 0
)

// MongoMember represents a MongoDB replica set member
type MongoMember struct {
	Host     string `json:"host"`
	StateStr string `json:"stateStr,omitempty"`
	Name     string `json:"name,omitempty"`
}

// ReplicaSetConfig represents MongoDB replica set configuration
type ReplicaSetConfig struct {
	Members []MongoMember `json:"members"`
}

// ReplicaSetStatus represents MongoDB replica set status
type ReplicaSetStatus struct {
	Members []MongoMember `json:"members"`
}

// MigrationContext holds context for MongoDB migration operations
type MigrationContext struct {
	StatefulSet  appsv1.StatefulSet
	Service      v1.Service
	PrimaryHost  string
	OriginHosts  []string
	UpdatedHosts []string
	TargetHosts  []string
}
