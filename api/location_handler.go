package api

import (
	"context"
	"encoding/json"
	"net/http"

	"SqlBuilder/db"
	"SqlBuilder/model"
	"SqlBuilder/querybuilder"
	"SqlBuilder/queryexec"
)

var pool = db.MustPool(context.Background())

func LocationsHandler(w http.ResponseWriter, r *http.Request) {
	var req querybuilder.ListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := queryexec.Fetch(r.Context(), pool,
		model.LocationQueryBuilder{}, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(rows)
	if err != nil {
		return
	}
}
