package inference

import (
	"context"
	"testing"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
)

type fakeProvider struct {
	name      string
	fail      bool
	retryable bool
	calls     int
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Generate(_ context.Context, req provider.Request) (provider.Response, error) {
	f.calls++
	if f.fail {
		return provider.Response{}, &provider.ProviderError{
			Provider:  f.name,
			Kind:      provider.ErrorRateLimit,
			Message:   "failure",
			Retryable: f.retryable,
		}
	}
	return provider.Response{Text: "success", Model: req.Model.Model}, nil
}

func TestFailoverToNextProvider(t *testing.T) {
	registry := provider.NewRegistry()
	primary := &fakeProvider{name: "primary", fail: true, retryable: true}
	secondary := &fakeProvider{name: "secondary"}
	registry.Register(primary)
	registry.Register(secondary)

	service := New(registry)
	response, selected, err := service.Generate(context.Background(), []provider.ModelRef{
		{Provider: "primary", Model: "model-a"},
		{Provider: "secondary", Model: "model-b"},
	}, provider.Request{Input: "hello"})
	if err != nil { t.Fatal(err) }
	if response.Text != "success" || selected.Provider != "secondary" {
		t.Fatalf("unexpected failover result: response=%+v model=%+v", response, selected)
	}
	if primary.calls != 2 || secondary.calls != 1 {
		t.Fatalf("unexpected call counts: primary=%d secondary=%d", primary.calls, secondary.calls)
	}
}

func TestNonRetryableErrorStopsFailover(t *testing.T) {
	registry := provider.NewRegistry()
	primary := &fakeProvider{name: "primary", fail: true, retryable: false}
	secondary := &fakeProvider{name: "secondary"}
	registry.Register(primary)
	registry.Register(secondary)

	service := New(registry)
	_, _, err := service.Generate(context.Background(), []provider.ModelRef{
		{Provider: "primary", Model: "model-a"},
		{Provider: "secondary", Model: "model-b"},
	}, provider.Request{Input: "hello"})
	if err == nil { t.Fatal("expected error") }
	if primary.calls != 1 || secondary.calls != 0 {
		t.Fatalf("non-retryable error should stop failover: primary=%d secondary=%d", primary.calls, secondary.calls)
	}
}
