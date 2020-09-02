package service

import (
	"context"
	"sync"
)

type Service interface {
	Run(valueCtx context.Context, serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup)
}
