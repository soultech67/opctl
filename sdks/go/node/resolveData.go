package node

import (
	"context"
	"log/slog"

	"github.com/opctl/opctl/sdks/go/data"
	"github.com/opctl/opctl/sdks/go/data/fs"
	"github.com/opctl/opctl/sdks/go/data/git"
	"github.com/opctl/opctl/sdks/go/model"
)

// Resolve attempts to resolve data via local filesystem or git
// nil pullCreds will be ignored
//
// expected errs:
//   - ErrDataProviderAuthentication on authentication failure
//   - ErrDataProviderAuthorization on authorization failure
//   - ErrDataRefResolution on resolution failure
func (cr core) ResolveData(
	ctx context.Context,
	dataRef string,
	pullCreds *model.Creds,
) (
	model.DataHandle,
	error,
) {
	callerSuppliedCreds := pullCreds != nil
	if pullCreds == nil {
		if auth := cr.stateStore.TryGetAuth(dataRef); auth != nil {
			pullCreds = &auth.Creds
		}
	}

	// Auth-decision diagnostics (no secret values): whether this resolve runs
	// authenticated, and whether the credential came from the caller or was
	// injected from the stored-auth lookup above.
	slog.Debug("resolveData",
		"ref", dataRef,
		"credsPresent", pullCreds != nil,
		"callerSupplied", callerSuppliedCreds,
		"injectedFromStore", !callerSuppliedCreds && pullCreds != nil,
	)

	return data.Resolve(
		ctx,
		dataRef,
		fs.New(),
		git.New(cr.dataCachePath, pullCreds),
	)
}
