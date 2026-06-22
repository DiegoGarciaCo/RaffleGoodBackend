// Package auth handles better-auth session validation and carries the
// authenticated user through the request context.
package auth

import (
	"context"

	"github.com/google/uuid"
)

// Role is a user's role within a nonprofit.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// User is the authenticated principal attached to a request's context.
type User struct {
	ID    uuid.UUID
	Name  string
	Email string

	// NonprofitID is valid only if the user manages a nonprofit.
	NonprofitID   uuid.NullUUID
	NonprofitRole Role
}

// IsNonprofitMember reports whether the user manages any nonprofit.
func (u *User) IsNonprofitMember() bool {
	return u != nil && u.NonprofitID.Valid
}

// CanManageOrg reports whether the user can change org settings (owner/admin).
func (u *User) CanManageOrg() bool {
	return u != nil && (u.NonprofitRole == RoleOwner || u.NonprofitRole == RoleAdmin)
}

// IsOwner reports whether the user owns the org.
func (u *User) IsOwner() bool {
	return u != nil && u.NonprofitRole == RoleOwner
}

// ─── Context storage ───────────────────────────────────────────────────────────

// unexported key type prevents collisions with other packages' context values.
type ctxKey struct{}

// WithUser returns a copy of ctx carrying the authenticated user.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// UserFromContext returns the authenticated user, or (nil, false) if anonymous.
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(ctxKey{}).(*User)
	return u, ok
}

// MustUser returns the user or nil. Use only on routes guarded by RequireAuth,
// where a user is guaranteed to be present.
func MustUser(ctx context.Context) *User {
	u, _ := UserFromContext(ctx)
	return u
}
