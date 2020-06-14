package browserpool

import (
	"context"
	"time"

	pool "github.com/jolestar/go-commons-pool/v2"
	"go.uber.org/zap"
)

const maxBrowserAge = time.Second

type browserFactory struct {
	logger *zap.Logger
}

func (bf *browserFactory) MakeObject(ctx context.Context) (*pool.PooledObject, error) {
	browser, err := NewBrowser()
	if err != nil {
		return nil, err
	}

	if err := browser.Launch(ctx); err != nil {
		return nil, err
	}

	bf.logger.Debug("created browser", zap.String("browser", browser.ID))

	return pool.NewPooledObject(browser), nil
}

func (bf *browserFactory) DestroyObject(ctx context.Context, object *pool.PooledObject) error {
	browser := object.Object.(*Browser)

	bf.logger.Debug("closing browser", zap.String("browser", browser.ID))

	return browser.Close(ctx)
}

func (bf *browserFactory) ValidateObject(ctx context.Context, object *pool.PooledObject) bool {
	browser := object.Object.(*Browser)

	if time.Since(browser.CreationTime) > maxBrowserAge {
		bf.logger.Debug("releasing old browser", zap.String("browser", browser.ID))
		return false
	}

	return true
}

func (bf *browserFactory) ActivateObject(ctx context.Context, object *pool.PooledObject) error {
	return nil
}

func (bf *browserFactory) PassivateObject(context.Context, *pool.PooledObject) error {
	return nil
}
