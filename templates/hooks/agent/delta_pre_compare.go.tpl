    // Hack to ensure that reconcile loop triggers update for PrepareAgent call
	// if AgentStatus is not in PREPARED state.
	compareAgentStatus(delta, b.ko.Status.AgentStatus)

	comparePropertyOverrideConfiguration(delta, a, b)