package httpapi

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type forbiddenLogValue struct{ t *testing.T }

func (v forbiddenLogValue) LogValue() slog.Value {
	v.t.Fatal("evaluated untrusted value")
	return slog.StringValue("secret")
}

func TestSafeLogBoundary(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewSafeLogHandler(slog.NewJSONHandler(&buf, nil)))
	secrets := []string{"password-secret", "Cookie-secret", "Bearer-secret", "CSRF-secret", "provider-token-secret", "body-secret"}
	attrs := []any{"request_id", "server_123", "status", 401, "duration", time.Second, "error_code", "AUTHENTICATION_FAILED"}
	for _, secret := range secrets {
		attrs = append(attrs, secret, secret)
	}
	attrs = append(attrs, "nested", slog.GroupValue(slog.String("password", secrets[0])), "entity_id", forbiddenLogValue{t})
	logger.With(attrs...).InfoContext(context.Background(), strings.Join(secrets, " "), attrs...)
	logger.WithGroup("password").With("request_id", secrets[0]).Info(secrets[1], "status", 200)
	logger.Info("secret", "error_code", secrets[2], "request_id", "newline\nsecret", "status", slog.StringValue(secrets[3]))
	output := buf.String()
	for _, secret := range secrets {
		if strings.Contains(output, secret) {
			t.Fatalf("secret leaked: %s", output)
		}
	}
	for _, want := range []string{`"msg":"event"`, `"request_id":"server_123"`, `"status":401`, `"error_code":"AUTHENTICATION_FAILED"`, `"duration":1000000000`} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %s: %s", want, output)
		}
	}
	if strings.Contains(output, "nested") || strings.Contains(output, "newline") {
		t.Fatal(output)
	}
}
