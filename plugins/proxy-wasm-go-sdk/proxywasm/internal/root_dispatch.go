package internal

import (
	"errors"
	"fmt"
)

// DispatchOnRootContext changes both host and SDK context while registering a
// callout. Updating only the host leaves the SDK callback owned by the request.
func DispatchOnRootContext(dispatch func() (uint32, error)) (calloutID uint32, err error) {
	callerID := currentState.activeContextID
	rootID, ok := currentState.contextIDToRootID[callerID]
	if !ok {
		return 0, StatusToError(StatusBadArgument)
	}
	if callerID == rootID {
		return dispatch()
	}
	if err := StatusToError(ProxySetEffectiveContext(rootID)); err != nil {
		return 0, fmt.Errorf("select plugin context: %w", err)
	}
	currentState.setActiveContextID(rootID)
	defer func() {
		restoreErr := StatusToError(ProxySetEffectiveContext(callerID))
		currentState.setActiveContextID(callerID)
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore caller context: %w", restoreErr))
		}
	}()
	return dispatch()
}
