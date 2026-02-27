package auth

import (
	"log/slog"
	"net/http"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// tokenReviewClient is set at init time so the middleware can resolve bearer tokens.
var tokenReviewClient kubernetes.Interface

// SetTokenReviewClient configures the K8s client used by AuthMiddleware for TokenReview.
func SetTokenReviewClient(c kubernetes.Interface) {
	tokenReviewClient = c
}

// AuthMiddleware extracts user identity from headers and adds to request context.
// It checks X-Remote-User first (set by API server proxies), then falls back
// to resolving a Bearer token via the TokenReview API (used by the OpenShift
// console plugin proxy with authorization: UserToken).
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Remote-User")
		groupsHeader := r.Header.Get("X-Remote-Group")

		var groups []string
		if groupsHeader != "" {
			groups = strings.Split(groupsHeader, ",")
			for i, g := range groups {
				groups[i] = strings.TrimSpace(g)
			}
		}

		// If no X-Remote-User, try Bearer token via TokenReview
		if user == "" {
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if tokenReviewClient != nil {
					tr := &authenticationv1.TokenReview{
						Spec: authenticationv1.TokenReviewSpec{Token: token},
					}
					result, err := tokenReviewClient.AuthenticationV1().TokenReviews().Create(r.Context(), tr, metav1.CreateOptions{})
					if err != nil {
						slog.WarnContext(r.Context(), "TokenReview failed", "error", err)
					} else if result.Status.Authenticated {
						user = result.Status.User.Username
						groups = result.Status.User.Groups
						slog.InfoContext(r.Context(), "user authenticated via token", "user", user)
					} else {
						slog.WarnContext(r.Context(), "token not authenticated")
					}
				}
			}
		}

		userInfo := &UserInfo{
			Name:   user,
			Groups: groups,
		}

		ctx := WithUserInfo(r.Context(), userInfo)
		if user != "" {
			slog.DebugContext(ctx, "user authenticated", "user", user, "groups", groups)
		} else {
			slog.DebugContext(ctx, "no user information in request")
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireUserMiddleware ensures user is authenticated
func RequireUserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Remote-User")
		if user == "" {
			slog.WarnContext(r.Context(), "request without authentication")
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RBACMiddleware checks authorization for write operations
func RBACMiddleware(rbac *RBACChecker, verb string, resource string, namespace string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userInfo := GetUserFromContext(r.Context())
			if userInfo == nil || userInfo.Name == "" {
				slog.WarnContext(r.Context(), "missing user info for authorization check")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			allowed, err := rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, verb, resource, namespace)
			if err != nil {
				slog.ErrorContext(r.Context(), "authorization check failed", "error", err)
				http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				return
			}

			if !allowed {
				slog.WarnContext(r.Context(), "authorization denied",
					"user", userInfo.Name, "verb", verb, "resource", resource)
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
