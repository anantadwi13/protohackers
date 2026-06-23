package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/anantadwi13/protohackers/11-pest-control/proto"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	addr := ":8080"
	srv, err := proto.NewServer(addr, &PestControlHandler{
		asClientManager: &proto.MockASCM{},
	})
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
		return
	}

	wg.Go(func() {
		log.Printf("listening on %s", addr)
		err := srv.Listen()
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
			return
		}
	})

	<-ctx.Done()
	log.Println("shutting down")
	err = srv.Shutdown(context.TODO())
	if err != nil {
		log.Fatalf("failed to shutdown server: %v", err)
		return
	}
}

type PestControlHandler struct {
	asClientManager proto.AuthorityServerClientManager

	policyTracker     map[uint32]map[string]map[uint32]struct{} // siteId, species, policyId
	policyTrackerLock sync.Mutex
}

func (p *PestControlHandler) HandleSiteVisit(ctx context.Context, msg proto.MessageSiteVisit) error {
	asClient, err := p.asClientManager.GetClient(ctx, msg.Site)
	if err != nil {
		return err
	}
	targetPopulation := asClient.TargetPopulation(ctx)

	mapPopulation := make(map[string]uint32) // key => species, value => count
	for _, population := range msg.Populations.Value() {
		count, ok := mapPopulation[population.Species.Value()]
		if ok && count != population.Count.Value() {
			// return error
			return errors.New("conflicting population counts for species " + population.Species.Value())
		}
		mapPopulation[population.Species.Value()] = population.Count.Value()
	}

	for species, populationCount := range mapPopulation {
		species, populationCount := species, populationCount

		err = func() error {
			target, ok := targetPopulation[species]
			if !ok {
				// species is not controlled
				return nil
			}

			var (
				policyId            proto.PolicyId
				policiesToBeDeleted []uint32
			)
			if populationCount < target.Min.Value() {
				policy, err := asClient.CreatePolicy(ctx, proto.MessageCreatePolicy{
					Species: proto.Species(species),
					Action:  proto.PolicyActionConserve,
				})
				if err != nil {
					return err
				}
				policyId = policy.Policy
			} else if populationCount > target.Max.Value() {
				policy, err := asClient.CreatePolicy(ctx, proto.MessageCreatePolicy{
					Species: proto.Species(species),
					Action:  proto.PolicyActionCull,
				})
				if err != nil {
					return err
				}
				policyId = policy.Policy
			} else {
				// no action
				return nil
			}

			p.policyTrackerLock.Lock()
			if p.policyTracker == nil {
				p.policyTracker = make(map[uint32]map[string]map[uint32]struct{})
			}
			if p.policyTracker[msg.Site.Value()] == nil {
				p.policyTracker[msg.Site.Value()] = make(map[string]map[uint32]struct{})
			}
			if p.policyTracker[msg.Site.Value()][species] == nil {
				p.policyTracker[msg.Site.Value()][species] = make(map[uint32]struct{})
			}
			for pid := range p.policyTracker[msg.Site.Value()][species] {
				policiesToBeDeleted = append(policiesToBeDeleted, pid)
				delete(p.policyTracker[msg.Site.Value()][species], pid)
			}
			p.policyTracker[msg.Site.Value()][species][policyId.Value()] = struct{}{}
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
			return nil
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
