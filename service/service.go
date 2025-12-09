package service

import (
	"context"
	"sync"
)

type Service interface {
	Start(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error
}
