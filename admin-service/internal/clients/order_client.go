package clients

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

// OrderClient calls order-service internal APIs.
type OrderClient struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
}

func NewOrderClient(baseURL string) *OrderClient {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "order-service",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
	return &OrderClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Second},
		cb:      cb,
	}
}

// GetStats calls GET /api/v1/internal/orders/stats
func (c *OrderClient) GetStats() ([]byte, int, error) {
	body, err := c.cb.Execute(func() (interface{}, error) {
		resp, err := c.http.Get(c.baseURL + "/api/v1/internal/orders/stats")
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
