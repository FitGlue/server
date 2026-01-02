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
