package runner

// This file defines an interface for task queues used by the runner
import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/go-stack/stack"
	"github.com/karlmutch/errors"
)

// QueueTask encapsulates the metadata needed to handle requests on a queue.
//
type QueueTask struct {
	FQProject    string // A proprietary runner label for a project to uniquely identify it
	Project      string
	QueueType    string
	Subscription string
	Credentials  string
	Msg          []byte
	Handler      MsgHandler
}

// MsgHandler defines the function signature for a generic message handler for a specified queue implementation
//
type MsgHandler func(ctx context.Context, qt *QueueTask) (resource *Resource, ack bool)

// TaskQueue is the interface definition for a queue message handling implementation.
//
type TaskQueue interface {
	// Refresh is used to scan the catalog of queues work could arrive on and pass them back to the caller
	Refresh(ctx context.Context, qNameMatch *regexp.Regexp) (known map[string]interface{}, err errors.Error)

	// Process a unit of work after it arrives on a queue, blocking operation on the queue and on the processing
	// of the work itself
	Work(ctx context.Context, qt *QueueTask) (msgs uint64, resource *Resource, err errors.Error)

	// Check that the specified queue exists
	Exists(ctx context.Context, subscription string) (exists bool, err errors.Error)
}

// NewTaskQueue is used to initiate processing for any of the types of queues
// the runner supports.  It also performs some lazy initialization.
//
func NewTaskQueue(project string, creds string) (tq TaskQueue, err errors.Error) {

	// The Google creds will come down as .json files, AWS will be a number of credential and config file names
	switch {
	case strings.HasSuffix(creds, ".json"):
		return NewPubSub(project, creds)
	case strings.HasPrefix(project, "amqp://"):
		return NewRabbitMQ(project, creds)
	default:
		files := strings.Split(creds, ",")
		for _, file := range files {
			_, errGo := os.Stat(file)
			if errGo != nil {
				return nil, errors.Wrap(errGo).With("stack", stack.Trace().TrimRuntime()).With("file", file).With("project", project)
			}
		}
		return NewSQS(project, creds)
	}
}
