    // If AgentStatus is not in PREPARED state we need to call PrepareAgent to finalize setup
    // of Agent. Uses hack (see delta.go) to trigger update from non-existent Spec.AgentStatus
    if delta.DifferentAt("Spec.AgentStatus") {
		prepareAgent(ctx, rm.sdkapi, rm.metrics, *desired.ko.Status.AgentID)
	}

	if delta.DifferentAt("Spec.Tags") {
		err := rm.syncTags(
			ctx,
			desired,
			latest,
		)
		if err != nil {
			return nil, err
		}
	}

	if !delta.DifferentExcept("Spec.AgentStatus", "Spec.Tags") {
		return desired, nil
	}
	