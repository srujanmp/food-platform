package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

// AuthClient calls auth-service internal APIs.
type AuthClient struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
}

func NewAuthClient(baseURL string) *AuthClient {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "auth-service",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
	return &AuthClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Second},
		cb:      cb,
	}
}

// BanUser calls POST /api/v1/internal/ban/:userId to revoke tokens.
func (c *AuthClient) BanUser(userID string) (json.RawMessage, int, error) {
	body, err := c.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/internal/ban/"+userID, nil)
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
