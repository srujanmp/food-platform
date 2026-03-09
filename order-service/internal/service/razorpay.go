package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// RazorpayClient wraps minimal Razorpay APIs used by order-service.
type RazorpayClient interface {
	CreateOrder(receipt string, amountPaise int64, currency string) (*RazorpayOrderResponse, error)
	VerifyPaymentSignature(orderID, paymentID, signature string) bool
	VerifyWebhookSignature(body []byte, signature string) bool
}

type razorpayClient struct {
	keyID         string
	keySecret     string
	webhookSecret string
	httpClient    *http.Client
}

type RazorpayOrderResponse struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Receipt  string `json:"receipt"`
}

func NewRazorpayClient(keyID, keySecret, webhookSecret string, httpClient *http.Client) RazorpayClient {
	return &razorpayClient{
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
		httpClient:    httpClient,
	}
}

func (c *razorpayClient) CreateOrder(receipt string, amountPaise int64, currency string) (*RazorpayOrderResponse, error) {
	if c.keyID == "" || c.keySecret == "" {
		return nil, errors.New("razorpay_not_configured")
	}

	payload := map[string]any{
		"amount":   amountPaise,
		"currency": currency,
		"receipt":  receipt,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.keyID, c.keySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("razorpay create order failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	var out RazorpayOrderResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *razorpayClient) VerifyPaymentSignature(orderID, paymentID, signature string) bool {
	if c.keySecret == "" {
		return false
	}
	signed := fmt.Sprintf("%s|%s", orderID, paymentID)
	h := hmac.New(sha256.New, []byte(c.keySecret))
	h.Write([]byte(signed))
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (c *razorpayClient) VerifyWebhookSignature(body []byte, signature string) bool {
	if c.webhookSecret == "" {
		return false
	}
	h := hmac.New(sha256.New, []byte(c.webhookSecret))
	h.Write(body)
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
