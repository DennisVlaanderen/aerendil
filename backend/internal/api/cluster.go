package api

import "net/http"

// Cluster status is Admin-only: it exposes node addresses and Raft internals.
func registerClusterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cluster/status", requireAdmin(handleErrors(clusterStatusHandler)))
}

func clusterStatusHandler(w http.ResponseWriter, _ *http.Request) error {
	status, err := dataStore.ClusterStatus()
	if err != nil {
		return err
	}
	return ok(w, status)
}
