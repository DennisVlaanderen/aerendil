package api

import (
	"net/http"

	"aerendil/backend/internal/auth"
)

func registerClusterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cluster/status", requirePermission(auth.PermClusterRead, handleErrors(clusterStatusHandler)))
}

func clusterStatusHandler(w http.ResponseWriter, r *http.Request) error {
	status, err := dataStore.ClusterStatus()
	if err != nil {
		return err
	}
	return ok(w, status)
}
