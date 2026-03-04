package execution

import (
	"context"

	"github.com/egokernel/ek1/internal/datasync"
)

// Executor performs a real action against a service API.
// Each executor handles one service slug.
type Executor interface {
	Slug() string
	Execute(ctx context.Context, creds datasync.Credentials, action Action) error
}
