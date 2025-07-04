package statefulset

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"time"
)

const (
	mongoPort            = "27017"
	mongoImage           = "mongo"
	mongoshCommand       = "mongosh"
	primaryState         = "PRIMARY"
	secondaryState       = "SECONDARY"
	defaultTimeout       = 10 * time.Minute
	defaultCheckInterval = 5 * time.Second
	stepDownDuration     = 60
	highPriority         = 1
	lowPriority          = 0
	// MongoDB client pod constants
	mongoClientPodName = "mongosh-client"
	mongoClientImage   = "mongo:latest"
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
