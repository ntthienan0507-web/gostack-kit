package broker

import (
	"fmt"
	"sync"
)

// Topic is a typed Kafka topic name.
type Topic string

// KeyStrategy defines how message keys are derived for a topic.
type KeyStrategy int

const (
	// KeyByID uses the entity ID as the message key.
	KeyByID KeyStrategy = iota
	// KeyByUserID uses the user ID as the message key.
	KeyByUserID
	// KeyByTenantID uses the tenant ID as the message key.
	KeyByTenantID
	// KeyNone means no key is required.
	KeyNone
)

// TopicMeta holds metadata about a registered topic.
type TopicMeta struct {
	Name        Topic
	EventType   string
	KeyStrategy KeyStrategy
}

var (
	registryMu sync.RWMutex
	registry   = make(map[Topic]TopicMeta)
)

// RegisterTopic registers a topic with its metadata.
// Returns an error if the topic is already registered.
func RegisterTopic(name Topic, eventType string, keyStrategy KeyStrategy) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[name]; exists {
		return fmt.Errorf("broker: topic %q is already registered", name)
	}

	registry[name] = TopicMeta{
		Name:        name,
		EventType:   eventType,
		KeyStrategy: keyStrategy,
	}
	return nil
}

// MustRegisterTopic registers a topic and panics on duplicate registration.
// Intended for use during initialization (e.g., package init or main).
func MustRegisterTopic(name Topic, eventType string, keyStrategy KeyStrategy) {
	if err := RegisterTopic(name, eventType, keyStrategy); err != nil {
		panic(err)
	}
}

// IsRegistered returns true if the topic has been registered.
func IsRegistered(name Topic) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}

// GetMeta returns the metadata for a registered topic.
// Returns an error if the topic is not registered.
func GetMeta(name Topic) (TopicMeta, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	meta, ok := registry[name]
	if !ok {
		return TopicMeta{}, fmt.Errorf("broker: topic %q is not registered", name)
	}
	return meta, nil
}

// ValidateKey checks that a non-empty key is provided when the topic's
// key strategy requires one. Returns an error if validation fails.
func ValidateKey(topic Topic, key string) error {
	registryMu.RLock()
	meta, ok := registry[topic]
	registryMu.RUnlock()

	if !ok {
		return fmt.Errorf("broker: topic %q is not registered", topic)
	}

	if meta.KeyStrategy != KeyNone && key == "" {
		return fmt.Errorf("broker: topic %q requires a key (strategy: %d)", topic, meta.KeyStrategy)
	}

	return nil
}

// ListTopics returns all registered topics.
func ListTopics() []TopicMeta {
	registryMu.RLock()
	defer registryMu.RUnlock()

	topics := make([]TopicMeta, 0, len(registry))
	for _, meta := range registry {
		topics = append(topics, meta)
	}
	return topics
}
