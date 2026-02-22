package auth

import (
	"fmt"
)

// Role represents a user role in the system
type Role string

const (
	RoleViewer      Role = "viewer"
	RoleDeveloper   Role = "developer"
	RolePolicyAdmin Role = "policy_admin"
	RoleApprover    Role = "approver"
	RoleOrgAdmin    Role = "org_admin"
)

// Permission represents a specific permission
type Permission string

const (
	// Run permissions
	PermRunView   Permission = "run:view"
	PermRunCreate Permission = "run:create"
	PermRunCancel Permission = "run:cancel"

	// Policy permissions
	PermPolicyView    Permission = "policy:view"
	PermPolicyCreate  Permission = "policy:create"
	PermPolicyEdit    Permission = "policy:edit"
	PermPolicyApprove Permission = "policy:approve"
	PermPolicyDeploy  Permission = "policy:deploy"

	// Evidence permissions
	PermEvidenceView   Permission = "evidence:view"
	PermEvidenceExport Permission = "evidence:export"

	// Admin permissions
	PermUserManage Permission = "user:manage"
	PermKeyManage  Permission = "key:manage"
	PermAuditView  Permission = "audit:view"
)

// RolePermissions maps roles to their permissions
var RolePermissions = map[Role][]Permission{
	RoleViewer: {
		PermRunView,
		PermPolicyView,
		PermEvidenceView,
	},
	RoleDeveloper: {
		PermRunView,
		PermRunCreate,
		PermPolicyView,
		PermEvidenceView,
		PermEvidenceExport,
	},
	RolePolicyAdmin: {
		PermRunView,
		PermPolicyView,
		PermPolicyCreate,
		PermPolicyEdit,
		PermEvidenceView,
	},
	RoleApprover: {
		PermRunView,
		PermPolicyView,
		PermPolicyApprove,
		PermEvidenceView,
	},
	RoleOrgAdmin: {
		PermRunView,
		PermRunCreate,
		PermRunCancel,
		PermPolicyView,
		PermPolicyCreate,
		PermPolicyEdit,
		PermPolicyApprove,
		PermPolicyDeploy,
		PermEvidenceView,
		PermEvidenceExport,
		PermUserManage,
		PermKeyManage,
		PermAuditView,
	},
}

// RBAC provides role-based access control
type RBAC struct{}

func NewRBAC() *RBAC {
	return &RBAC{}
}

// HasPermission checks if a role has a specific permission
func (r *RBAC) HasPermission(role Role, permission Permission) bool {
	permissions, exists := RolePermissions[role]
	if !exists {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}

	return false
}

// RequirePermission returns an error if the role lacks the permission
func (r *RBAC) RequirePermission(role Role, permission Permission) error {
	if !r.HasPermission(role, permission) {
		return fmt.Errorf("role '%s' lacks permission '%s'", role, permission)
	}
	return nil
}

// GetPermissions returns all permissions for a role
func (r *RBAC) GetPermissions(role Role) []Permission {
	return RolePermissions[role]
}

// ValidateRole checks if a role string is valid
func (r *RBAC) ValidateRole(roleStr string) (Role, error) {
	role := Role(roleStr)
	if _, exists := RolePermissions[role]; !exists {
		return "", fmt.Errorf("invalid role: %s", roleStr)
	}
	return role, nil
}

// IsValidRole returns true when role is a known role with permissions.
func (r *RBAC) IsValidRole(role Role) bool {
	_, exists := RolePermissions[role]
	return exists
}
