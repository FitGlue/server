package firestore

import (
	"cloud.google.com/go/firestore"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type Client struct {
	fs *firestore.Client
}

func NewClient(client *firestore.Client) *Client {
	return &Client{fs: client}
}

func (c *Client) Close() error {
	return c.fs.Close()
}

func (c *Client) Users() *Collection[pb.UserRecord] {
	return &Collection[pb.UserRecord]{
		Ref:           c.fs.Collection("users"),
		ToFirestore:   UserToFirestore,
		FromFirestore: FirestoreToUser,
	}
}

func (c *Client) Executions() *Collection[pb.ExecutionRecord] {
	return &Collection[pb.ExecutionRecord]{
		Ref:           c.fs.Collection("executions"),
		ToFirestore:   ExecutionToFirestore,
		FromFirestore: FirestoreToExecution,
	}
}

func (c *Client) PendingInputs() *Collection[pb.PendingInput] {
	return &Collection[pb.PendingInput]{
		Ref:           c.fs.Collection("pending_inputs"),
		ToFirestore:   PendingInputToFirestore,
		FromFirestore: FirestoreToPendingInput,
	}
}

// Counters are sub-collections of Users: users/{uid}/counters/{id}
func (c *Client) Counters(userId string) *Collection[pb.Counter] {
	return &Collection[pb.Counter]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("counters"),
		ToFirestore:   CounterToFirestore,
		FromFirestore: FirestoreToCounter,
	}
}

// Activities are sub-collections of Users: users/{uid}/activities/{id}
func (c *Client) Activities(userId string) *Collection[pb.SynchronizedActivity] {
	return &Collection[pb.SynchronizedActivity]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("activities"),
		ToFirestore:   SynchronizedActivityToFirestore,
		FromFirestore: FirestoreToSynchronizedActivity,
	}
}
