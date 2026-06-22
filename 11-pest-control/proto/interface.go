package proto

import (
	"context"
)

type Server interface {
	Listen() error
	Shutdown(ctx context.Context) error

	internalProtoImplementation
}

type ServerHandler interface {
	HandleSiteVisit(ctx context.Context, msg MessageSiteVisit)
}

type AuthorityServerClientManager interface {
	GetClient(ctx context.Context, siteId SiteId) (AuthorityServerClient, error)
	Shutdown(ctx context.Context) error

	internalProtoImplementation
}

type TargetPopulation map[string]TargetPopulationsPopulation // key => species

type AuthorityServerClient interface {
	TargetPopulation(ctx context.Context) TargetPopulation
	CreatePolicy(ctx context.Context, msg MessageCreatePolicy) (MessagePolicyResult, error)
	DeletePolicy(ctx context.Context, msg MessageDeletePolicy) error
	Shutdown(ctx context.Context) error

	internalProtoImplementation
}

type internalProtoImplementation interface {
	internalProtoImplementation()
}
