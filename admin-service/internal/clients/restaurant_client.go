package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

// RestaurantClient calls restaurant-service APIs.
type RestaurantClient struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
}

func NewRestaurantClient(baseURL string) *RestaurantClient {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "restaurant-service",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
	return &RestaurantClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Second},
		cb:      cb,
	}
}

// ListRestaurants calls GET /api/v1/internal/restaurants (all restaurants, including unapproved)
func (c *RestaurantClient) ListRestaurants() ([]byte, int, error) {
	body, err := c.cb.Execute(func() (interface{}, error) {
		resp, err := c.http.Get(c.baseURL + "/api/v1/internal/restaurants")
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

// ApproveRestaurant calls PATCH /api/v1/restaurants/:id/approve with the admin JWT.
func (c *RestaurantClient) ApproveRestaurant(id string, adminToken string) (json.RawMessage, int, error) {
	body, err := c.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodPatch, c.baseURL+"/api/v1/restaurants/"+id+"/approve", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+adminToken)
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
