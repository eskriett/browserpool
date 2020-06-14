package browserpool

import (
	"context"

	pool "github.com/jolestar/go-commons-pool/v2"
	"go.uber.org/zap"
)

type Pool struct {
	factory    *browserFactory
	objectPool *pool.ObjectPool
}

func New(ctx context.Context) (*Pool, error) {
	p := &Pool{}

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	p.factory = &browserFactory{logger: logger}
	objectPoolCfg := pool.NewDefaultPoolConfig()
	objectPoolCfg.TestOnReturn = true
	objectPoolCfg.MaxTotal = 5

	objectPool := pool.NewObjectPool(ctx, p.factory, objectPoolCfg)
	p.objectPool = objectPool

	return p, nil
}

func (p *Pool) GetBrowser(ctx context.Context) (*Browser, error) {
	obj, err := p.objectPool.BorrowObject(ctx)
	if err != nil {
		return nil, err
	}

	browser := obj.(*Browser)

	return browser, err
}

func (p *Pool) ReleaseBrowser(ctx context.Context, b *Browser) error {
	return p.objectPool.ReturnObject(ctx, b)
}

func (p *Pool) Close(ctx context.Context) {
	p.objectPool.Close(ctx)
}
