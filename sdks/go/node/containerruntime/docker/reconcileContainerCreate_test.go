package docker

import (
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/opctl/opctl/sdks/go/model"
	. "github.com/opctl/opctl/sdks/go/node/containerruntime/docker/internal/fakes"
)

// Tests for reconcileTimedOutContainerCreate — the safety net that catches
// orphan Created-state containers when dockerd completes a ContainerCreate
// after our 20s timeout fires. See runContainer.go for the motivation.

func TestReconcileTimedOutContainerCreateNoOpOnNilOrEmptyReq(t *testing.T) {
	cases := []struct {
		name string
		req  *model.ContainerCall
	}{
		{name: "nil req", req: nil},
		{name: "empty ContainerID", req: &model.ContainerCall{ContainerID: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := new(FakeCommonAPIClient)
			reconcileTimedOutContainerCreate(fakeClient, tc.req, "opctl_x_y")
			if fakeClient.ContainerListCallCount() != 0 {
				t.Fatalf("expected no ContainerList calls; got %d", fakeClient.ContainerListCallCount())
			}
			if fakeClient.ContainerRemoveCallCount() != 0 {
				t.Fatalf("expected no ContainerRemove calls; got %d", fakeClient.ContainerRemoveCallCount())
			}
		})
	}
}

func TestReconcileTimedOutContainerCreateNoOrphanFound(t *testing.T) {
	fakeClient := new(FakeCommonAPIClient)
	fakeClient.ContainerListReturns([]types.Container{}, nil)

	reconcileTimedOutContainerCreate(
		fakeClient,
		&model.ContainerCall{ContainerID: "callid-123"},
		"opctl_image_callid-123",
	)

	if fakeClient.ContainerListCallCount() != 1 {
		t.Fatalf("expected 1 ContainerList call; got %d", fakeClient.ContainerListCallCount())
	}
	// Confirm the list was filtered by our container-id label.
	_, listOpts := fakeClient.ContainerListArgsForCall(0)
	if !listOpts.All {
		t.Fatalf("expected ContainerList with All=true; got All=%v", listOpts.All)
	}
	want := getContainerIDLabelFilter("callid-123")
	if !listOpts.Filters.ExactMatch("label", want) {
		t.Fatalf("expected ContainerList filter label=%q; got %+v", want, listOpts.Filters)
	}
	if fakeClient.ContainerRemoveCallCount() != 0 {
		t.Fatalf("expected NO ContainerRemove calls when no orphan; got %d", fakeClient.ContainerRemoveCallCount())
	}
}

func TestReconcileTimedOutContainerCreateRemovesFoundOrphan(t *testing.T) {
	fakeClient := new(FakeCommonAPIClient)
	fakeClient.ContainerListReturns([]types.Container{
		{
			ID:    "orphan-container-id",
			Names: []string{"/opctl_image_callid-123"},
			State: "created",
		},
	}, nil)
	fakeClient.ContainerRemoveReturns(nil)

	reconcileTimedOutContainerCreate(
		fakeClient,
		&model.ContainerCall{ContainerID: "callid-123"},
		"opctl_image_callid-123",
	)

	if fakeClient.ContainerRemoveCallCount() != 1 {
		t.Fatalf("expected 1 ContainerRemove call; got %d", fakeClient.ContainerRemoveCallCount())
	}
	_, removeID, removeOpts := fakeClient.ContainerRemoveArgsForCall(0)
	if removeID != "orphan-container-id" {
		t.Fatalf("expected ContainerRemove(orphan-container-id); got %q", removeID)
	}
	if !removeOpts.Force {
		t.Fatalf("expected Force=true on reconcile remove; got Force=%v", removeOpts.Force)
	}
	if !removeOpts.RemoveVolumes {
		t.Fatalf("expected RemoveVolumes=true on reconcile remove; got %v", removeOpts.RemoveVolumes)
	}
}

func TestReconcileTimedOutContainerCreateRemovesEveryOrphan(t *testing.T) {
	// Theoretically the label lookup should match at most one container, but
	// if Docker somehow returns more (e.g. duplicate label by mistake) we
	// want to clean up every match rather than leak any.
	fakeClient := new(FakeCommonAPIClient)
	fakeClient.ContainerListReturns([]types.Container{
		{ID: "orphan-a", State: "created"},
		{ID: "orphan-b", State: "created"},
		{ID: "orphan-c", State: "exited"},
	}, nil)
	fakeClient.ContainerRemoveReturns(nil)

	reconcileTimedOutContainerCreate(
		fakeClient,
		&model.ContainerCall{ContainerID: "callid-123"},
		"opctl_image_callid-123",
	)

	if fakeClient.ContainerRemoveCallCount() != 3 {
		t.Fatalf("expected ContainerRemove called 3x; got %d", fakeClient.ContainerRemoveCallCount())
	}
	got := []string{}
	for i := 0; i < 3; i++ {
		_, id, _ := fakeClient.ContainerRemoveArgsForCall(i)
		got = append(got, id)
	}
	for _, want := range []string{"orphan-a", "orphan-b", "orphan-c"} {
		found := false
		for _, id := range got {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected ContainerRemove called with id %q; got calls=%v", want, got)
		}
	}
}

func TestReconcileTimedOutContainerCreateBailsCleanlyOnListError(t *testing.T) {
	fakeClient := new(FakeCommonAPIClient)
	fakeClient.ContainerListReturns(nil, errors.New("docker is still wedged"))

	reconcileTimedOutContainerCreate(
		fakeClient,
		&model.ContainerCall{ContainerID: "callid-123"},
		"opctl_image_callid-123",
	)

	// On a list failure we can't know if there's an orphan; bail rather than
	// guessing. `opctl container prune` is the recovery path the log line
	// points at.
	if fakeClient.ContainerRemoveCallCount() != 0 {
		t.Fatalf("expected NO ContainerRemove calls when ContainerList errs; got %d", fakeClient.ContainerRemoveCallCount())
	}
}

func TestReconcileTimedOutContainerCreateTreatsExpectedRemoveErrorsAsSuccess(t *testing.T) {
	cases := []struct {
		name      string
		removeErr error
	}{
		{
			name:      "not-found (container already gone)",
			removeErr: errdefs.NotFound(errors.New("No such container: orphan-x")),
		},
		{
			name:      "conflict-in-progress (concurrent removal)",
			removeErr: errdefs.Conflict(errors.New("removal of container orphan-x is already in progress")),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := new(FakeCommonAPIClient)
			fakeClient.ContainerListReturns([]types.Container{
				{ID: "orphan-x", State: "created"},
			}, nil)
			fakeClient.ContainerRemoveReturns(tc.removeErr)

			// Just call it — if these errors aren't handled, we'd see a
			// panic or log spam. The contract is: reconcile doesn't itself
			// fail just because the underlying remove no-op'd.
			reconcileTimedOutContainerCreate(
				fakeClient,
				&model.ContainerCall{ContainerID: "callid-123"},
				"opctl_image_callid-123",
			)

			if fakeClient.ContainerRemoveCallCount() != 1 {
				t.Fatalf("expected 1 ContainerRemove call; got %d", fakeClient.ContainerRemoveCallCount())
			}
		})
	}
}

