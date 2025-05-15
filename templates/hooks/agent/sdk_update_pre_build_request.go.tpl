    // If AgentStatus is not in PREPARED state we need to call PrepareAgent to finalize setup
    // of Agent. Uses hack (see delta.go) to trigger update from non-existent Spec.AgentStatus
    if delta.DifferentAt("Spec.AgentStatus") {
		prepareAgent(ctx, rm.sdkapi, rm.metrics, *desired.ko.Status.AgentID)
	}

	if !delta.DifferentExcept("Spec.AgentStatus") {
		return desired, nil
	}