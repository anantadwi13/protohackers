package main

import (
	"context"
	"sync"

	"github.com/anantadwi13/protohackers/11-pest-control/proto"
)

type PestControlHandler struct {
	asClientManager proto.AuthorityServerClientManager

	policyTracker     map[uint32]map[string]map[uint32]struct{} // siteId, species, policyId
	policyTrackerLock sync.Mutex
}

func (p *PestControlHandler) HandleSiteVisit(ctx context.Context, msg proto.MessageSiteVisit) {
	asClient, err := p.asClientManager.GetClient(ctx, msg.Site)
	if err != nil {
		return
	}
	targetPopulation := asClient.TargetPopulation(ctx)

	for _, population := range msg.Populations.Value() {
		population := population

		func() {
			target, ok := targetPopulation[population.Species.Value()]
			if !ok {
				// species is not controlled
				return
			}

			var (
				policyId            proto.PolicyId
				policiesToBeDeleted []uint32
			)
			if population.Count.Value() < target.Min.Value() {
				policy, err := asClient.CreatePolicy(ctx, proto.MessageCreatePolicy{
					Species: population.Species,
					Action:  proto.PolicyActionConserve,
				})
				if err != nil {
					return
				}
				policyId = policy.Policy
			} else if population.Count.Value() > target.Max.Value() {
				policy, err := asClient.CreatePolicy(ctx, proto.MessageCreatePolicy{
					Species: population.Species,
					Action:  proto.PolicyActionCull,
				})
				if err != nil {
					return
				}
				policyId = policy.Policy
			} else {
				// no action
				return
			}

			p.policyTrackerLock.Lock()
			if p.policyTracker == nil {
				p.policyTracker = make(map[uint32]map[string]map[uint32]struct{})
			}
			if p.policyTracker[msg.Site.Value()] == nil {
				p.policyTracker[msg.Site.Value()] = make(map[string]map[uint32]struct{})
			}
			if p.policyTracker[msg.Site.Value()][population.Species.Value()] == nil {
				p.policyTracker[msg.Site.Value()][population.Species.Value()] = make(map[uint32]struct{})
			}
			for pid := range p.policyTracker[msg.Site.Value()][population.Species.Value()] {
				policiesToBeDeleted = append(policiesToBeDeleted, pid)
				delete(p.policyTracker[msg.Site.Value()][population.Species.Value()], pid)
			}
			p.policyTracker[msg.Site.Value()][population.Species.Value()][policyId.Value()] = struct{}{}
			p.policyTrackerLock.Unlock()

			for _, pid := range policiesToBeDeleted {
				// todo: concurrently delete policies?
				err := asClient.DeletePolicy(ctx, proto.MessageDeletePolicy{
					Policy: proto.PolicyId(pid),
				})
				if err != nil {
					// log the error but continue
					// it is expected that some policies may be already deleted
					// todo: but how to check if this is because of the policy already deleted or some other reason (network error, etc)?
				}
			}
		}()
	}
}
