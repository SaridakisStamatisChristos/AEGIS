package contracts

import (
	"fmt"
	"regexp"
)

var ulidPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// ValidateULID checks if a string is a valid ULID
func ValidateULID(id string) error {
	if !ulidPattern.MatchString(id) {
		return fmt.Errorf("invalid ULID format: %s", id)
	}
	return nil
}

// ValidateRunStatus checks if a run status is valid
func ValidateRunStatus(status string) error {
	validStatuses := []RunStatus{
		RunStatusRunning,
		RunStatusCompleted,
		RunStatusFailed,
		RunStatusBlocked,
		RunStatusCancelled,
	}

	for _, s := range validStatuses {
		if RunStatus(status) == s {
			return nil
		}
	}

	return fmt.Errorf("invalid run status: %s", status)
}

// ValidatePolicyAction checks if a policy action is valid
func ValidatePolicyAction(action string) error {
	validActions := []PolicyAction{
		ActionAllow,
		ActionWarn,
		ActionRedact,
		ActionBlock,
		ActionRequireApproval,
		ActionDegrade,
	}

	for _, a := range validActions {
		if PolicyAction(action) == a {
			return nil
		}
	}

	return fmt.Errorf("invalid policy action: %s", action)
}
