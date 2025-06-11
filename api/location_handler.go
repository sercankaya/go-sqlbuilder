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

	rows, count, err := queryexec.Fetch(r.Context(), pool,
		model.LocationQueryBuilder{}, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Rows  []map[string]any `json:"rows"`
		Count int64            `json:"count"`
	}{
		Rows:  rows,
		Count: count,
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		return
	}
}
