package auth

import (
	"log/slog"
	"net/http"
	"strings"
)

// AuthMiddleware extracts user identity from headers and adds to request context
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
