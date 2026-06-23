package proto

import (
	"context"
	"log"
)

type MockASCM struct {
}

func (A *MockASCM) GetClient(ctx context.Context, siteId SiteId) (AuthorityServerClient, error) {
	return &MockASC{}, nil
}

func (A *MockASCM) Shutdown(ctx context.Context) error {
	return nil
}

func (A *MockASCM) internalProtoImplementation() {
	// no-op
}

type MockASC struct {
}

func (A *MockASC) TargetPopulation(ctx context.Context) TargetPopulation {
	return TargetPopulation{
		"cat": TargetPopulationsPopulation{
			Species: "cat",
			Min:     0,
			Max:     10,
		},
	}
}

func (A *MockASC) CreatePolicy(ctx context.Context, msg MessageCreatePolicy) (MessagePolicyResult, error) {
	log.Printf("CreatePolicy: %v", msg)
	return MessagePolicyResult{Policy: 12345}, nil
}

func (A *MockASC) DeletePolicy(ctx context.Context, msg MessageDeletePolicy) error {
	log.Printf("DeletePolicy: %v", msg)
	return nil
}

func (A *MockASC) Shutdown(ctx context.Context) error {
	return nil
}

func (A *MockASC) internalProtoImplementation() {
	// no-op
}
