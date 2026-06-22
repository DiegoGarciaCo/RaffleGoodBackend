package handlers

import (
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

// HandleStripeWebhook receives Stripe events. It is authenticated by Stripe's
// signature (the Stripe-Signature header), NOT by a user session — so it's
// registered raw, outside the auth middleware.
//
//	POST /webhooks/stripe
//
// This is a skeleton: it reads + acknowledges the event. Wire real handling
// (payment_intent.succeeded, payout.paid, account.updated, charge.refunded)
// during the payments integration, verifying the signature with your Stripe
// webhook secret before trusting the payload.
func (cfg *apiCfg) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBody = 1 << 20 // 1 MB
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not read body")
		return
	}
	_ = r.Body.Close()

	sig := r.Header.Get("Stripe-Signature")
	if sig == "" {
		// In production, reject unsigned requests.
		logrus.Warn("stripe webhook: missing signature")
	}

	// TODO (payments step):
	//   event, err := webhook.ConstructEvent(payload, sig, cfg.stripeWebhookSecret)
	//   switch event.Type {
	//   case "payment_intent.succeeded": ... mark order paid
	//   case "payout.paid":              ... mark payout paid
	//   case "account.updated":          ... update Connect account / bank verified
	//   case "charge.refunded":          ... mark order refunded
	//   }
	logrus.WithField("bytes", len(payload)).Info("stripe webhook received (skeleton)")

	// Always 200 quickly so Stripe doesn't retry; real processing can be async.
	respondWithJSON(w, http.StatusOK, map[string]string{"received": "true"})
}
