package docker

import (
	"errors"
	"testing"

	"github.com/docker/docker/errdefs"
)

func TestClassifyExpectedDockerNoop(t *testing.T) {
	cases := []struct {
		name string
		op   string
		err  error
		want string
	}{
		{
			name: "nil error returns empty",
			op:   "ContainerStop",
			err:  nil,
			want: "",
		},
		{
			name: "ContainerStop on not-found container is noop",
			op:   "ContainerStop",
			err:  errdefs.NotFound(errors.New("No such container: opctl_x")),
			want: "not found",
		},
		{
			name: "ContainerRemove on not-found container is noop",
			op:   "ContainerRemove",
			err:  errdefs.NotFound(errors.New("No such container: opctl_x")),
			want: "not found",
		},
		{
			name: "ContainerRemove already-in-progress conflict is noop",
			op:   "ContainerRemove",
			err:  errdefs.Conflict(errors.New("removal of container opctl_x is already in progress")),
			want: "already in progress",
		},
		{
			name: "NetworkCreate on already-existing network is noop (plain string match)",
			op:   "NetworkCreate",
			err:  errors.New("Error response from daemon: network with name opctl already exists"),
			want: "already exists",
		},
		{
			name: "ContainerStop with unrelated error is NOT noop",
			op:   "ContainerStop",
			err:  errors.New("unexpected EOF"),
			want: "",
		},
		{
			name: "ContainerRemove with unrelated conflict is NOT noop",
			op:   "ContainerRemove",
			err:  errdefs.Conflict(errors.New("some other conflict")),
			want: "",
		},
		{
			name: "ContainerCreate not-found is NOT noop (only Stop/Remove get the demote)",
			op:   "ContainerCreate",
			err:  errdefs.NotFound(errors.New("not found")),
			want: "",
		},
		{
			name: "NetworkCreate with unrelated error is NOT noop",
			op:   "NetworkCreate",
			err:  errors.New("daemon unreachable"),
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyExpectedDockerNoop(tc.op, tc.err)
			if got != tc.want {
				t.Fatalf("classifyExpectedDockerNoop(%q, %v) = %q; want %q", tc.op, tc.err, got, tc.want)
			}
		})
	}
}
