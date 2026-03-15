package broker

import (
	"errors"

	"github.com/IBM/sarama"
)

// retryableErrors lists Kafka error codes that are transient and worth retrying.
var retryableErrors = map[sarama.KError]bool{
	sarama.ErrNotLeaderForPartition:         true,
	sarama.ErrLeaderNotAvailable:            true,
	sarama.ErrBrokerNotAvailable:            true,
	sarama.ErrReplicaNotAvailable:           true,
	sarama.ErrRequestTimedOut:               true,
	sarama.ErrNotEnoughReplicas:             true,
	sarama.ErrNotEnoughReplicasAfterAppend:  true,
	sarama.ErrNetworkException:              true,
	sarama.ErrOffsetsLoadInProgress:         true,
}

// permanentErrors lists Kafka error codes that are not retryable.
var permanentErrors = map[sarama.KError]bool{
	sarama.ErrMessageSizeTooLarge:      true,
	sarama.ErrInvalidMessage:           true,
	sarama.ErrTopicAuthorizationFailed: true,
	sarama.ErrUnknownTopicOrPartition:  true,
	sarama.ErrInvalidTopic:             true,
	sarama.ErrUnsupportedVersion:       true,
}

// IsRetryableKafkaError returns true if the error is a transient Kafka error
// that should be retried, false if it is a permanent error that should not
// be retried. For unknown errors, returns true (retry by default).
func IsRetryableKafkaError(err error) bool {
	if err == nil {
		return false
	}

	var kErr sarama.KError
	if errors.As(err, &kErr) {
		if permanentErrors[kErr] {
			return false
		}
		if retryableErrors[kErr] {
			return true
		}
	}

	// For non-Kafka errors or unknown Kafka errors, default to retryable.
	return true
}
