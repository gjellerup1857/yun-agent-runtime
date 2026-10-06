package inference

import (
	"context"
	"fmt"
	"time"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
)

type Service struct {
	providers           *provider.Registry
	maxAttemptsPerModel int
}

func New(providers *provider.Registry) *Service {
	return &Service{providers: providers, maxAttemptsPerModel: 2}
}

func (s *Service) Generate(
	ctx context.Context,
	candidates []provider.ModelRef,
	req provider.Request,
) (provider.Response, provider.ModelRef, error) {
	var lastErr error

	for _, model := range candidates {
		p, ok := s.providers.Get(model.Provider)
		if !ok {
			continue
		}

		for attempt := 0; attempt < s.maxAttemptsPerModel; attempt++ {
			if err := ctx.Err(); err != nil {
				return provider.Response{}, provider.ModelRef{}, err
			}

			req.Model = model
			response, err := p.Generate(ctx, req)
			if err == nil {
				return response, model, nil
			}

			lastErr = err
			if !provider.IsRetryable(err) {
				return provider.Response{}, provider.ModelRef{}, err
			}

			if attempt+1 < s.maxAttemptsPerModel {
				delay := time.Duration(250*(1<<attempt)) * time.Millisecond
				select {
				case <-ctx.Done():
					return provider.Response{}, provider.ModelRef{}, ctx.Err()
				case <-time.After(delay):
				}
			}
		}
	}

	if lastErr != nil {
		return provider.Response{}, provider.ModelRef{}, lastErr
	}
	return provider.Response{}, provider.ModelRef{}, fmt.Errorf("no registered inference provider available")
}
