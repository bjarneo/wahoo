package billing

import (
	"testing"
	"time"
)

func TestCheckoutRequestValidation(t *testing.T) {
	t.Parallel()
	request := CheckoutRequest{
		CustomerID: "cus_123",
		PriceID:    "price_team",
		SuccessURL: "https://app.example.com/billing/success",
		CancelURL:  "https://app.example.com/billing/cancel",
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.SuccessURL = "javascript:alert(1)"
	if err := request.Validate(); err != ErrInvalidRequest {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestUsageValidation(t *testing.T) {
	t.Parallel()
	usage := Usage{ID: "evt_123", CustomerID: "cus_123", Metric: "api_call", Quantity: 1, OccurredAt: time.Now()}
	if err := usage.Validate(); err != nil {
		t.Fatal(err)
	}
}
