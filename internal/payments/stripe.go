package payments

// Stripe client wrapper. Keep all stripe-go usage behind this package so the
// rest of the app depends on a small, testable surface.
//
// go get github.com/stripe/stripe-go/v79
// (if your go.mod pins a different major version, change the import paths to match)

import (
	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/paymentintent"
	"github.com/stripe/stripe-go/v79/webhook"
)

type Client struct {
	webhookSecret string
}

// NewClient sets the global Stripe API key and returns a client that also knows
// the webhook signing secret. Call once at startup.
func NewClient(secretKey, webhookSecret string) *Client {
	stripe.Key = secretKey
	return &Client{webhookSecret: webhookSecret}
}

type PaymentIntentInput struct {
	AmountCents int64
	Currency    string // defaults to "usd"
	Metadata    map[string]string
}

// CreatePaymentIntent creates a PaymentIntent with automatic payment methods
// (so the mobile PaymentSheet can offer cards + wallets). Returns the intent;
// the caller sends ClientSecret to the app.
func (c *Client) CreatePaymentIntent(in PaymentIntentInput) (*stripe.PaymentIntent, error) {
	cur := in.Currency
	if cur == "" {
		cur = "usd"
	}
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(in.AmountCents),
		Currency: stripe.String(cur),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	for k, v := range in.Metadata {
		params.AddMetadata(k, v)
	}
	return paymentintent.New(params)
}

// ConstructEvent verifies the webhook signature and returns the parsed event.
//
// IgnoreAPIVersionMismatch is deliberately set: stripe-go/v79 pins API version
// 2024-06-20, but this account (and the Stripe CLI) runs a newer version
// (e.g. 2025-03-31.basil). Without this flag, stripe-go rejects every event
// with an API-version-mismatch error that surfaces as a 400 — even though the
// HMAC signature is valid. The fields we consume (PaymentIntent.ID, .Status,
// .Metadata) are stable across these versions, so skipping the version-equality
// check is safe. The signature is still fully verified.
func (c *Client) ConstructEvent(payload []byte, sigHeader string) (stripe.Event, error) {
	return webhook.ConstructEventWithOptions(
		payload, sigHeader, c.webhookSecret,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
}
