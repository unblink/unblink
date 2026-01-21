package relay

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
)

type userIDKey struct{}

// AuthInterceptor implements connect.Interceptor for both unary and streaming calls
type AuthInterceptor struct {
	jwtManager *JWTManager
}

// NewAuthInterceptor creates a Connect interceptor that validates JWT tokens
func NewAuthInterceptor(jwtManager *JWTManager) connect.Interceptor {
	return &AuthInterceptor{jwtManager: jwtManager}
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		newCtx, err := i.authenticate(ctx, req.Header(), req.Spec().Procedure)
		if err != nil {
			return nil, err
		}
		return next(newCtx, req)
	})
}

func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		newCtx, err := i.authenticate(ctx, conn.RequestHeader(), conn.Spec().Procedure)
		if err != nil {
			return err
		}
		return next(newCtx, conn)
	})
}

func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *AuthInterceptor) authenticate(ctx context.Context, header http.Header, procedure string) (context.Context, error) {
	// Skip auth for Register and Login
	if procedure == "/unblink.auth.v1.AuthService/Register" ||
		procedure == "/unblink.auth.v1.AuthService/Login" {
		return ctx, nil
	}

	// Check for dev mode impersonation header first
	if impersonate := header.Get("X-Dev-Impersonate"); impersonate != "" {
		// In dev mode, allow impersonation with a mock user ID
		ctx = context.WithValue(ctx, userIDKey{}, "00000000-0000-0000-0000-000000000001")
		return ctx, nil
	}

	// Extract token from request header
	authHeader := header.Get("Authorization")
	if authHeader == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			errors.New("missing authorization header"))
	}

	// Parse Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			errors.New("invalid authorization format"))
	}

	// Validate JWT
	claims, err := i.jwtManager.ValidateToken(parts[1])
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			errors.New("invalid token"))
	}

	// Add user ID to context
	ctx = context.WithValue(ctx, userIDKey{}, claims.UserID)
	return ctx, nil
}

// GetUserIDFromContext extracts the user ID from the request context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey{}).(string)
	return userID, ok
}
