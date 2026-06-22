package handlers

import "net/http"

// HandleListCategories returns the browse tree with item counts (counts include
// subcategories, via the v_category_item_counts view). Powers the Explore sidebar.
//
//	GET /categories
func (cfg *apiCfg) HandleListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := cfg.DB.ListCategoryItemCounts(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load categories")
		return
	}
	respondWithJSON(w, http.StatusOK, cats)
}

// HandleGetCategory returns one category by slug.
//
//	GET /categories/{slug}
func (cfg *apiCfg) HandleGetCategory(w http.ResponseWriter, r *http.Request) {
	cat, err := cfg.DB.GetCategoryBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		cfg.handleDBError(w, err, "category")
		return
	}
	respondWithJSON(w, http.StatusOK, cat)
}
