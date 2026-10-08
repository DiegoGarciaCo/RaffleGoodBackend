package handlers

// Nonprofit verification via CharityAPI. Looks up the org's EIN against IRS
// records, records the result in nonprofit_verifications, and sets hasToPay:
// churches (IRS foundation code 10) are fee-exempt; everyone else pays.

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/charity"
	"github.com/diegoGarciaCo/raffles/internal/database"
)

type verifyRequest struct {
	EIN string `json:"ein"`
}

type verifyResponse struct {
	IsVerified       bool   `json:"is_verified"`
	IsChurch         bool   `json:"is_church"`
	HasToPay         bool   `json:"has_to_pay"`
	OrganizationName string `json:"organization_name"`
	Subsection       string `json:"subsection"`
	NteeCd           string `json:"ntee_cd"`
}

// HandleVerifyNonprofit verifies the authed org against the IRS via CharityAPI.
//
//	POST /nonprofits/me/verify   (org only)
//	body: { "ein": "12-3456789" }
func (cfg *apiCfg) HandleVerifyNonprofit(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	orgID := nonprofitID(user)

	var req verifyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ein := charity.NormalizeEIN(req.EIN)
	if len(ein) != 9 {
		respondWithError(w, http.StatusBadRequest, "please enter a valid 9-digit EIN")
		return
	}

	org, found, err := cfg.Charity.Lookup(r.Context(), ein)
	if err != nil {
		respondWithError(w, http.StatusBadGateway, "could not reach the IRS verification service, please try again")
		return
	}
	if !found {
		respondWithError(w, http.StatusNotFound, "no tax-exempt organization found for that EIN")
		return
	}

	isChurch := org.IsChurch()
	hasToPay := !isChurch

	// API values → DB column types.
	subCode, subOK := org.SubsectionCode()
	statusCode, statusOK := org.StatusCode()
	foundCode, foundOK := org.FoundationCode()
	rulingTime, rulingOK := org.RulingTime()

	err = cfg.withTx(r.Context(), func(qtx *database.Queries) error {
		if _, err := qtx.CreateNonprofitVerification(r.Context(), database.CreateNonprofitVerificationParams{
			NonprofitID:        orgID,
			IsVerified:         sql.NullBool{Bool: true, Valid: true},
			VerificationMethod: nullStr("charityapi"),
			ExemptStatus:       nullInt32(statusCode, statusOK),
			Subsection:         nullInt32(subCode, subOK),
			NteeCd:             nullStr(org.NteeCd),
			RulingDate:         sql.NullTime{Time: rulingTime, Valid: rulingOK},
			Foundation:         nullInt32(foundCode, foundOK),
		}); err != nil {
			return err
		}
		if _, err := qtx.SetNonprofitEIN(r.Context(), database.SetNonprofitEINParams{
			ID:  orgID,
			Ein: nullStr(ein),
		}); err != nil {
			return err
		}
		if _, err := qtx.SetNonprofitHasToPay(r.Context(), database.SetNonprofitHasToPayParams{
			ID:       orgID,
			HasToPay: hasToPay,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not save verification")
		return
	}

	respondWithJSON(w, http.StatusOK, verifyResponse{
		IsVerified:       true,
		IsChurch:         isChurch,
		HasToPay:         hasToPay,
		OrganizationName: org.Name,
		Subsection:       strings.TrimSpace(org.Subsection.String()),
		NteeCd:           org.NteeCd,
	})
}
