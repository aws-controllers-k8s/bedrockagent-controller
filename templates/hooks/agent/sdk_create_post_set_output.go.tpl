    // Agent is created in NOT_PREPARED state which is not fully ready.
    // To complete setup need to call PrepareAgent. Note this call may
    // fail as Agent has been created recently. Will need to ensure that 
    // subsequent reconcile loops retry if Agent is not PREPARED.
    prepareAgent(ctx, rm.sdkapi, rm.metrics, *ko.Status.AgentID)
    