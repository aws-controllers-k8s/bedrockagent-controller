    // Hack to ensure that reconcile loop triggers update for PrepareAgent call
	// if AgentStatus is not in PREPARED state.
	compareAgentStatus(b.ko.Status.AgentStatus, delta)