import { AERENDIL_API_ORIGIN as API_ORIGIN } from '$app/env/private';

// Mirrors the backend's store.ClusterStatus / store.NodeInfo.
export interface ClusterNode {
	id: string;
	address: string;
	suffrage: string;
	leader: boolean;
	lag: number | null;
}

export interface ClusterStatus {
	leaderId: string;
	leaderAddress: string;
	nodes: ClusterNode[];
	commitIndex: number;
	lastLogIndex: number;
	term: number;
}

// Returns null when the backend fails, so the page can show an error state
// instead of a misleading empty cluster.
export async function getClusterStatus(token: string): Promise<ClusterStatus | null> {
	const response = await fetch(`${API_ORIGIN}/api/cluster/status`, {
		headers: { Authorization: `Bearer ${token}` }
	}).catch(() => null);
	if (!response?.ok) {
		console.error(`getClusterStatus: backend returned ${response?.status ?? 'no response'}`);
		return null;
	}
	const payload = await response.json().catch(() => null);
	return Array.isArray(payload?.nodes) ? payload : null;
}
