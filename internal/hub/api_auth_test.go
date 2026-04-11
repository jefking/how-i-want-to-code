package hub

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveAgentToken_UsesConfiguredAgentTokenWhenVerified(t *testing.T) {
	t.Parallel()

	var bindCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agents/me":
			if r.Header.Get("Authorization") == "Bearer agent_saved" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		case "/v1/agents/bind":
			bindCalls++
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client := NewAPIClient(server.URL + "/v1")
	token, err := client.ResolveAgentToken(context.Background(), InitConfig{
		AgentToken: "agent_saved",
		BindToken:  "bind_unused",
	})
	if err != nil {
		t.Fatalf("ResolveAgentToken() error = %v", err)
	}
	if token != "agent_saved" {
		t.Fatalf("ResolveAgentToken() token = %q, want %q", token, "agent_saved")
	}
	if bindCalls != 0 {
		t.Fatalf("expected bind flow not to run, bindCalls=%d", bindCalls)
	}
}

func TestResolveAgentToken_AgentTokenUnverifiedFallsBackToConfiguredBindToken(t *testing.T) {
	t.Parallel()

	var bindBodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agents/bind":
			data, _ := io.ReadAll(r.Body)
			bindBodies = append(bindBodies, string(data))
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("bind Authorization = %q, want empty", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"agent_token":"agent_bound"}`))
		case "/v1/agents/me":
			if r.Header.Get("Authorization") == "Bearer agent_bound" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client := NewAPIClient(server.URL + "/v1")
	token, err := client.ResolveAgentToken(context.Background(), InitConfig{
		AgentToken: "agent_saved",
		BindToken:  "bind_valid",
	})
	if err != nil {
		t.Fatalf("ResolveAgentToken() error = %v", err)
	}
	if token != "agent_bound" {
		t.Fatalf("ResolveAgentToken() token = %q, want %q", token, "agent_bound")
	}
	if len(bindBodies) == 0 {
		t.Fatal("expected bind fallback attempt")
	}
	if got := strings.TrimSpace(bindBodies[0]); got != `{"bind_token":"bind_valid"}` {
		t.Fatalf("first bind attempt body = %q, want canonical bind_token payload", got)
	}
}

func TestResolveAgentToken_AgentTokenUnverifiedAttemptsAgentTokenAsBindCandidate(t *testing.T) {
	t.Parallel()

	var bindCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agents/bind":
			bindCalls++
			data, _ := io.ReadAll(r.Body)
			if strings.Contains(string(data), "agent_saved") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"agent_token":"agent_bound"}`))
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		case "/v1/agents/me":
			if r.Header.Get("Authorization") == "Bearer agent_bound" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client := NewAPIClient(server.URL + "/v1")
	token, err := client.ResolveAgentToken(context.Background(), InitConfig{
		AgentToken: "agent_saved",
	})
	if err != nil {
		t.Fatalf("ResolveAgentToken() error = %v", err)
	}
	if token != "agent_bound" {
		t.Fatalf("ResolveAgentToken() token = %q, want %q", token, "agent_bound")
	}
	if bindCalls == 0 {
		t.Fatal("expected bind candidate fallback using agent token")
	}
}

func TestResolveAgentToken_AgentTokenUnverifiedReturnsOriginalTokenWhenFallbackFails(t *testing.T) {
	t.Parallel()

	var bindCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agents/me":
			w.WriteHeader(http.StatusUnauthorized)
		case "/v1/agents/bind":
			bindCalls++
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client := NewAPIClient(server.URL + "/v1")
	token, err := client.ResolveAgentToken(context.Background(), InitConfig{
		AgentToken: "agent_saved",
		BindToken:  "bind_invalid",
	})
	if err != nil {
		t.Fatalf("ResolveAgentToken() error = %v", err)
	}
	if token != "agent_saved" {
		t.Fatalf("ResolveAgentToken() token = %q, want %q", token, "agent_saved")
	}
	if bindCalls == 0 {
		t.Fatal("expected bind fallback attempts before using original token")
	}
}

func TestResolveAgentToken_BindFallbackAllowsUnverifiedToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agents/bind":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"agent_token":"agent_bound"}`))
		case "/v1/agents/me":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client := NewAPIClient(server.URL + "/v1")
	token, err := client.ResolveAgentToken(context.Background(), InitConfig{
		BindToken: "bind_valid",
	})
	if err != nil {
		t.Fatalf("ResolveAgentToken() error = %v", err)
	}
	if token != "agent_bound" {
		t.Fatalf("ResolveAgentToken() token = %q, want %q", token, "agent_bound")
	}
}

func TestResolveAgentToken_RequiresSomeCredential(t *testing.T) {
	t.Parallel()

	client := NewAPIClient("https://na.hub.molten.bot/v1")
	_, err := client.ResolveAgentToken(context.Background(), InitConfig{})
	if err == nil {
		t.Fatalf("ResolveAgentToken() expected error")
	}
	if !strings.Contains(err.Error(), "missing bind_token and agent_token") {
		t.Fatalf("ResolveAgentToken() err = %q", err)
	}
}

func TestResolveAgentToken_BindTokenFailureIncludesHubErrorDetails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agents/bind" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_request","message":"invalid JSON request"}`))
	}))
	t.Cleanup(server.Close)

	client := NewAPIClient(server.URL + "/v1")
	_, err := client.ResolveAgentToken(context.Background(), InitConfig{
		BindToken: "bind_invalid",
	})
	if err == nil {
		t.Fatal("ResolveAgentToken() error = nil, want non-nil")
	}
	for _, want := range []string{
		"bind flow failed for provided bind_token",
		"/agents/bind: hub API 400 invalid_request: invalid JSON request",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ResolveAgentToken() err = %q, want containing %q", err, want)
		}
	}
}
