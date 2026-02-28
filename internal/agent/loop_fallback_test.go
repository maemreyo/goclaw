package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

type scriptedProvider struct {
	calls       []string
	failByModel map[string]error
}

func (p *scriptedProvider) Chat(_ context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	p.calls = append(p.calls, req.Model)
	if err, ok := p.failByModel[req.Model]; ok {
		return nil, err
	}
	return &providers.ChatResponse{Content: "ok", FinishReason: "stop"}, nil
}

func (p *scriptedProvider) ChatStream(ctx context.Context, req providers.ChatRequest, onChunk func(providers.StreamChunk)) (*providers.ChatResponse, error) {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	if onChunk != nil {
		onChunk(providers.StreamChunk{Content: resp.Content})
		onChunk(providers.StreamChunk{Done: true})
	}
	return resp, nil
}

func (p *scriptedProvider) DefaultModel() string { return "" }
func (p *scriptedProvider) Name() string         { return "openrouter" }

func TestCallProviderWithFallback_OnRateLimitUsesNextModel(t *testing.T) {
	prov := &scriptedProvider{failByModel: map[string]error{
		"m1": &providers.HTTPError{Status: 429, Body: "rate limit"},
	}}
	loop := &Loop{
		id:             "router-agent",
		provider:       prov,
		model:          "m1",
		modelFallbacks: []string{"m2"},
	}

	resp, usedModel, err := loop.callProviderWithFallback(context.Background(), providers.ChatRequest{Model: "m1"}, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usedModel != "m2" {
		t.Fatalf("usedModel = %q, want %q", usedModel, "m2")
	}
	if resp == nil || resp.Content != "ok" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	wantCalls := []string{"m1", "m2"}
	if !reflect.DeepEqual(prov.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", prov.calls, wantCalls)
	}
}

func TestCallProviderWithFallback_NonRateLimitDoesNotFallback(t *testing.T) {
	prov := &scriptedProvider{failByModel: map[string]error{
		"m1": &providers.HTTPError{Status: 400, Body: "bad request"},
	}}
	loop := &Loop{
		id:             "router-agent",
		provider:       prov,
		model:          "m1",
		modelFallbacks: []string{"m2"},
	}

	_, usedModel, err := loop.callProviderWithFallback(context.Background(), providers.ChatRequest{Model: "m1"}, false, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if usedModel != "m1" {
		t.Fatalf("usedModel = %q, want %q", usedModel, "m1")
	}
	wantCalls := []string{"m1"}
	if !reflect.DeepEqual(prov.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", prov.calls, wantCalls)
	}
}

func TestModelCandidates_DedupesAndKeepsOrder(t *testing.T) {
	loop := &Loop{
		modelFallbacks: []string{"m1", "", "m2", "m2", "m3"},
	}
	got := loop.modelCandidates("m1")
	want := []string{"m1", "m2", "m3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %#v, want %#v", got, want)
	}
}

func TestIsRateLimitFailure_RecognizesWrappedHTTP429(t *testing.T) {
	err := errors.New("wrapper: " + (&providers.HTTPError{Status: 429, Body: "too many requests"}).Error())
	if !isRateLimitFailure(err) {
		t.Fatal("expected wrapped 429-like error to be treated as rate-limit")
	}
}
