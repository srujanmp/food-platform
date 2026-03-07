package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

// UserClient calls user-service internal APIs.
type UserClient struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
}

func NewUserClient(baseURL string) *UserClient {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "user-service",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
	return &UserClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Second},
		cb:      cb,
	}
}

// ListUsers calls GET /api/v1/internal/users and returns the raw JSON.
func (c *UserClient) ListUsers() ([]byte, int, error) {
	body, err := c.cb.Execute(func() (interface{}, error) {
		resp, err := c.http.Get(c.baseURL + "/api/v1/internal/users")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return &httpResult{body: b, status: resp.StatusCode}, nil
	})
	if err != nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("upstream service unavailable: %w", err)
	}
	result := body.(*httpResult)
	return result.body, result.status, nil
}

// BanUser calls PATCH /api/v1/internal/users/:id/ban
func (c *UserClient) BanUser(userID string) (json.RawMessage, int, error) {
	body, err := c.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodPatch, c.baseURL+"/api/v1/internal/users/"+userID+"/ban", nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return &httpResult{body: b, status: resp.StatusCode}, nil
	})
	if err != nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("upstream service unavailable: %w", err)
	}
	result := body.(*httpResult)
	return result.body, result.status, nil
}

type httpResult struct {
	body   []byte
	status int
}
