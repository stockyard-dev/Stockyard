package apiserver

import (
	"sync"
	"testing"
)

// TestClaimWebhookEvent_FirstClaimWins covers the happy path: a single
// caller claims an event, gets (true, nil). A second call with the
// same event_id returns (false, nil) — the second caller knows to
// skip processing.
func TestClaimWebhookEvent_FirstClaimWins(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	claimed, err := db.ClaimWebhookEvent("evt_abc", "checkout.session.completed")
	if err != nil {
		t.Fatalf("first claim: unexpected error: %v", err)
	}
	if !claimed {
		t.Fatal("first claim should have succeeded")
	}

	claimed2, err := db.ClaimWebhookEvent("evt_abc", "checkout.session.completed")
	if err != nil {
		t.Fatalf("second claim: unexpected error: %v", err)
	}
	if claimed2 {
		t.Fatal("second claim should have been rejected (already claimed)")
	}
}

// TestClaimWebhookEvent_DifferentIDsCoexist: claims on distinct
// event IDs should all succeed.
func TestClaimWebhookEvent_DifferentIDsCoexist(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	for _, id := range []string{"evt_1", "evt_2", "evt_3"} {
		claimed, err := db.ClaimWebhookEvent(id, "test")
		if err != nil {
			t.Fatalf("claim %s: %v", id, err)
		}
		if !claimed {
			t.Fatalf("claim %s should have succeeded", id)
		}
	}
}

// TestUnclaimWebhookEvent_ReleasesForRetry covers the failure-recovery
// path: if processing fails and we unclaim, a subsequent retry can
// re-claim the same event_id.
func TestUnclaimWebhookEvent_ReleasesForRetry(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// First claim succeeds.
	claimed, _ := db.ClaimWebhookEvent("evt_retry", "test")
	if !claimed {
		t.Fatal("initial claim failed")
	}

	// Simulate processing failure: unclaim.
	if err := db.UnclaimWebhookEvent("evt_retry"); err != nil {
		t.Fatalf("unclaim: %v", err)
	}

	// Retry can now re-claim.
	reclaim, err := db.ClaimWebhookEvent("evt_retry", "test")
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if !reclaim {
		t.Fatal("reclaim after unclaim should have succeeded")
	}
}

// TestUnclaimWebhookEvent_NoOpOnMissing: unclaiming a never-claimed
// event_id should not error. Safe to call blindly.
func TestUnclaimWebhookEvent_NoOpOnMissing(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := db.UnclaimWebhookEvent("evt_never_existed"); err != nil {
		t.Fatalf("unclaim of absent event should be no-op, got: %v", err)
	}
}

// TestClaimWebhookEvent_ConcurrentRace is the big one. 5 goroutines
// race to claim the same event_id. Exactly ONE should succeed; the
// other 4 should get claimed=false. This is the scenario that
// previously double-minted licenses under the old check-then-act
// pattern, and the whole point of adding Claim/Unclaim.
//
// 5 is the realistic upper bound on Stripe concurrent deliveries for
// the same event_id in production: automatic retry + manual replay
// from the dashboard + webhookrelay tool overlap + two API instances
// behind a load balancer = ~5 simultaneous callers. We deliberately
// don't hammer with hundreds of goroutines because that exceeds the
// real production threat model and puts us in SQLite lock-contention
// territory that isn't what this test is about.
func TestClaimWebhookEvent_ConcurrentRace(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	const N = 5
	var wg sync.WaitGroup
	successes := make(chan bool, N)
	errors := make(chan error, N)

	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			ok, err := db.ClaimWebhookEvent("evt_race", "test")
			if err != nil {
				errors <- err
				return
			}
			successes <- ok
		}()
	}
	wg.Wait()
	close(successes)
	close(errors)

	// Any DB error is a test failure — the code path should distinguish
	// "someone else has it" (claimed=false, no error) from "DB broken".
	for err := range errors {
		t.Errorf("unexpected DB error during race: %v", err)
	}

	winCount := 0
	for ok := range successes {
		if ok {
			winCount++
		}
	}
	if winCount != 1 {
		t.Fatalf("expected exactly 1 claim winner across %d racers, got %d", N, winCount)
	}
}

// TestClaimWebhookEvent_IsWebhookProcessedConsistent verifies the
// legacy IsWebhookProcessed method still reflects the new Claim-
// based state. Some test code (and potentially non-webhook callers)
// might still query it.
func TestClaimWebhookEvent_IsWebhookProcessedConsistent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if db.IsWebhookProcessed("evt_consistency") {
		t.Fatal("should not be processed before claim")
	}

	claimed, _ := db.ClaimWebhookEvent("evt_consistency", "test")
	if !claimed {
		t.Fatal("claim failed")
	}
	if !db.IsWebhookProcessed("evt_consistency") {
		t.Fatal("after claim, IsWebhookProcessed should report true")
	}

	db.UnclaimWebhookEvent("evt_consistency")
	if db.IsWebhookProcessed("evt_consistency") {
		t.Fatal("after unclaim, IsWebhookProcessed should report false")
	}
}
