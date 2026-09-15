package scheduler

import (
	"context"
	"testing"
)

// Manual triggers arrive with an HTTP request context that net/http cancels
// when the handler returns. Runs must survive that; only server shutdown
// may interrupt them.
func TestExecBaseSurvivesRequestCancel(t *testing.T) {
	st := testStore(t)
	s := New("", st, nil)
	s.stopCtx = context.Background() // simulate started server

	reqCtx, cancel := context.WithCancel(context.Background())
	base := s.execBase(reqCtx)
	cancel()

	select {
	case <-base.Done():
		t.Fatal("run context died with the HTTP request")
	default:
	}

	select {
	case <-reqCtx.Done():
	default:
		t.Fatal("test setup broken: request ctx should be cancelled")
	}
}

func TestExecBaseDetachesBeforeStart(t *testing.T) {
	st := testStore(t)
	s := New("", st, nil) // stopCtx nil: Trigger before Start

	reqCtx, cancel := context.WithCancel(context.Background())
	base := s.execBase(reqCtx)
	cancel()

	if base.Err() != nil {
		t.Fatal("run context must detach from the request context")
	}
}

func TestExecBaseFollowsServerShutdown(t *testing.T) {
	st := testStore(t)
	s := New("", st, nil)
	srvCtx, stop := context.WithCancel(context.Background())
	s.stopCtx = srvCtx

	reqCtx := context.Background()
	base := s.execBase(reqCtx)
	stop()

	select {
	case <-base.Done():
	default:
		t.Fatal("run context must cancel on server shutdown")
	}
}
