package broker

import (
	"crypto/sha256"
	"crypto/sha512"

	"github.com/xdg-go/scram"
)

// SHA256 is a hash generator for SCRAM-SHA-256.
var SHA256 scram.HashGeneratorFcn = sha256.New

// SHA512 is a hash generator for SCRAM-SHA-512.
var SHA512 scram.HashGeneratorFcn = sha512.New

// scramClient implements sarama.SCRAMClient using xdg-go/scram.
type scramClient struct {
	HashGeneratorFcn scram.HashGeneratorFcn
	client           *scram.Client
	conversation     *scram.ClientConversation
}

func (sc *scramClient) Begin(userName, password, authzID string) error {
	client, err := sc.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}

	sc.client = client
	sc.conversation = client.NewConversation()
	return nil
}

func (sc *scramClient) Step(challenge string) (string, error) {
	return sc.conversation.Step(challenge)
}

func (sc *scramClient) Done() bool {
	return sc.conversation.Done()
}
