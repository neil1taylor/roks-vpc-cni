package auth

import (
	"context"
	"fmt"
	"log/slog"

	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RBACChecker provides RBAC authorization checks
type RBACChecker struct {
	clientset kubernetes.Interface
}

// NewRBACChecker creates a new RBAC checker
func NewRBACChecker(clientset kubernetes.Interface) *RBACChecker {
	return &RBACChecker{
		clientset: clientset,
	}
}

// CheckAccess verifies if a user can perform an action on a resource
func (r *RBACChecker) CheckAccess(ctx context.Context, user string, groups []string,
	verb string, resource string, namespace string) (bool, error) {

	if user == "" {
		slog.WarnContext(ctx, "empty user in authorization check")
		return false, fmt.Errorf("user cannot be empty")
	}

	// Create SubjectAccessReview
	sar := &authzv1.SubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: authzv1.SubjectAccessReviewSpec{
			User:   user,
			Groups: groups,
			ResourceAttributes: &authzv1.ResourceAttributes{
				Group:     "vpc.ibm.com",
				Version:   "v1alpha1",
				Resource:  resource,
				Namespace: namespace,
				Verb:      verb,
			},
		},
	}

	// Submit SAR to API server
	result, err := r.clientset.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to check authorization",
			"user", user, "verb", verb, "resource", resource, "error", err)
		return false, fmt.Errorf("failed to check authorization: %w", err)
	}

	if result.Status.Allowed {
		slog.InfoContext(ctx, "authorization allowed",
			"user", user, "verb", verb, "resource", resource)
		return true, nil
	}

	slog.InfoContext(ctx, "authorization denied",
		"user", user, "verb", verb, "resource", resource,
		"reason", result.Status.Reason)

	return false, nil
}

// UserInfo holds user identity information
type UserInfo struct {
	Name   string
	Groups []string
}

// GetUserFromContext extracts user info from context
func GetUserFromContext(ctx context.Context) *UserInfo {
	val := ctx.Value(userInfoContextKey)
	if val == nil {
		return nil
	}
	return val.(*UserInfo)
}

// ContextKey type for user info in context
type contextKeyType string

const userInfoContextKey contextKeyType = "user-info"

// WithUserInfo adds user info to context
func WithUserInfo(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, userInfoContextKey, user)
}
