package auth

import (
	databasev1 "weave/core/gen/database/v1"
)

// roleToString maps the proto Role enum to the plain string shared-auth's
// JWT claims carry — shared-auth is deliberately decoupled from core's
// generated proto types so it stays reusable by non-Go/non-core services.
func roleToString(r databasev1.Role) string {
	switch r {
	case databasev1.Role_ROLE_OWNER:
		return "owner"
	case databasev1.Role_ROLE_ADMIN:
		return "admin"
	case databasev1.Role_ROLE_STAFF:
		return "staff"
	case databasev1.Role_ROLE_CUSTOMER:
		return "customer"
	default:
		return ""
	}
}
