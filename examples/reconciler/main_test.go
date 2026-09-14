package main

import (
	"context"
	"testing"
)

func TestReconcilerExample(t *testing.T) {
	// Test main() execution
	main()

	r := &ClusterReconciler{}
	dag, err := r.BuildDAG()
	if err != nil {
		t.Fatalf("unexpected error building DAG: %v", err)
	}

	ctx := context.Background()

	// 1. Test provisioning path (not deleted, not ready)
	state1 := &ClusterState{
		ClusterID:    "test-cluster-1",
		Namespace:    "default",
		NodeCount:    3,
		IsDeleted:    false,
		IsReady:      false,
		StorageClass: "standard",
		Endpoints:    []string{"10.0.0.1:8080"},
	}
	if err := dag.Exec(ctx, state1); err != nil {
		t.Fatalf("provision path exec failed: %v", err)
	}

	// 2. Test already ready path (not deleted, ready)
	state2 := &ClusterState{
		ClusterID: "test-cluster-2",
		IsDeleted: false,
		IsReady:   true,
	}
	if err := dag.Exec(ctx, state2); err != nil {
		t.Fatalf("ready path exec failed: %v", err)
	}

	// 3. Test deletion path (is deleted)
	state3 := &ClusterState{
		ClusterID: "test-cluster-3",
		IsDeleted: true,
	}
	if err := dag.Exec(ctx, state3); err != nil {
		t.Fatalf("deletion path exec failed: %v", err)
	}

	// 4. Test error predicate and failure steps
	_ = isTransientQuotaError(ctx, nil)
	_ = r.scheduleRequeue(ctx, state1)
	_ = r.collectDiagnosticLogs(ctx, state1)
	_ = r.rollbackUnhealthyNodes(ctx, state1)
	_ = r.markClusterDegraded(ctx, state1)
}
