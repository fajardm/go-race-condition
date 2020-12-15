package locker

import (
	"context"
	"errors"
	"time"
)

var ErrLocked = errors.New("still locked")

type Locker interface {
	Lock(ctx context.Context, key string, expiration time.Duration) error
	Unlock(ctx context.Context, key string) error
	Locked(ctx context.Context, key string) bool
}
