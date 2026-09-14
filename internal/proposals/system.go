package proposals

import "context"

type System interface {
	Process(ctx context.Context, prop Proposal) error
}
