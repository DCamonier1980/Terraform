package terraform

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/plans"
	"github.com/hashicorp/terraform/plans/objchange"
	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/zclconf/go-cty/cty"
)

// NodeAbstractResourceInstance represents a resource instance with no
// associated operations. It embeds NodeAbstractResource but additionally
// contains an instance key, used to identify one of potentially many
// instances that were created from a resource in configuration, e.g. using
// the "count" or "for_each" arguments.
type NodeAbstractResourceInstance struct {
	NodeAbstractResource
	Addr addrs.AbsResourceInstance

	// These are set via the AttachState method.
	instanceState *states.ResourceInstance
	// storedProviderConfig is the provider address retrieved from the
	// state, but since it is only stored in the whole Resource rather than the
	// ResourceInstance, we extract it out here.
	storedProviderConfig addrs.AbsProviderConfig

	Dependencies []addrs.ConfigResource
}

// NewNodeAbstractResourceInstance creates an abstract resource instance graph
// node for the given absolute resource instance address.
func NewNodeAbstractResourceInstance(addr addrs.AbsResourceInstance) *NodeAbstractResourceInstance {
	// Due to the fact that we embed NodeAbstractResource, the given address
	// actually ends up split between the resource address in the embedded
	// object and the InstanceKey field in our own struct. The
	// ResourceInstanceAddr method will stick these back together again on
	// request.
	r := NewNodeAbstractResource(addr.ContainingResource().Config())
	return &NodeAbstractResourceInstance{
		NodeAbstractResource: *r,
		Addr:                 addr,
	}
}

func (n *NodeAbstractResourceInstance) Name() string {
	return n.ResourceInstanceAddr().String()
}

func (n *NodeAbstractResourceInstance) Path() addrs.ModuleInstance {
	return n.Addr.Module
}

// GraphNodeReferenceable
func (n *NodeAbstractResourceInstance) ReferenceableAddrs() []addrs.Referenceable {
	addr := n.ResourceInstanceAddr()
	return []addrs.Referenceable{
		addr.Resource,

		// A resource instance can also be referenced by the address of its
		// containing resource, so that e.g. a reference to aws_instance.foo
		// would match both aws_instance.foo[0] and aws_instance.foo[1].
		addr.ContainingResource().Resource,
	}
}

// GraphNodeReferencer
func (n *NodeAbstractResourceInstance) References() []*addrs.Reference {
	// If we have a configuration attached then we'll delegate to our
	// embedded abstract resource, which knows how to extract dependencies
	// from configuration. If there is no config, then the dependencies will
	// be connected during destroy from those stored in the state.
	if n.Config != nil {
		if n.Schema == nil {
			// We'll produce a log message about this out here so that
			// we can include the full instance address, since the equivalent
			// message in NodeAbstractResource.References cannot see it.
			log.Printf("[WARN] no schema is attached to %s, so config references cannot be detected", n.Name())
			return nil
		}
		return n.NodeAbstractResource.References()
	}

	// If we have neither config nor state then we have no references.
	return nil
}

// StateDependencies returns the dependencies saved in the state.
func (n *NodeAbstractResourceInstance) StateDependencies() []addrs.ConfigResource {
	if s := n.instanceState; s != nil {
		if s.Current != nil {
			return s.Current.Dependencies
		}
	}

	return nil
}

// GraphNodeProviderConsumer
func (n *NodeAbstractResourceInstance) ProvidedBy() (addrs.ProviderConfig, bool) {
	// If we have a config we prefer that above all else
	if n.Config != nil {
		relAddr := n.Config.ProviderConfigAddr()
		return addrs.LocalProviderConfig{
			LocalName: relAddr.LocalName,
			Alias:     relAddr.Alias,
		}, false
	}

	// See if we have a valid provider config from the state.
	if n.storedProviderConfig.Provider.Type != "" {
		// An address from the state must match exactly, since we must ensure
		// we refresh/destroy a resource with the same provider configuration
		// that created it.
		return n.storedProviderConfig, true
	}

	// No provider configuration found; return a default address
	return addrs.AbsProviderConfig{
		Provider: n.Provider(),
		Module:   n.ModulePath(),
	}, false
}

// GraphNodeProviderConsumer
func (n *NodeAbstractResourceInstance) Provider() addrs.Provider {
	if n.Config != nil {
		return n.Config.Provider
	}
	return addrs.ImpliedProviderForUnqualifiedType(n.Addr.Resource.ContainingResource().ImpliedProvider())
}

// GraphNodeResourceInstance
func (n *NodeAbstractResourceInstance) ResourceInstanceAddr() addrs.AbsResourceInstance {
	return n.Addr
}

// GraphNodeAttachResourceState
func (n *NodeAbstractResourceInstance) AttachResourceState(s *states.Resource) {
	if s == nil {
		log.Printf("[WARN] attaching nil state to %s", n.Addr)
		return
	}
	n.instanceState = s.Instance(n.Addr.Resource.Key)
	n.storedProviderConfig = s.ProviderConfig
}

// readDiff returns the planned change for a particular resource instance
// object.
func (n *NodeAbstractResourceInstance) readDiff(ctx EvalContext, providerSchema *ProviderSchema) (*plans.ResourceInstanceChange, error) {
	changes := ctx.Changes()
	addr := n.ResourceInstanceAddr()

	schema, _ := providerSchema.SchemaForResourceAddr(addr.Resource.Resource)
	if schema == nil {
		// Should be caught during validation, so we don't bother with a pretty error here
		return nil, fmt.Errorf("provider does not support resource type %q", addr.Resource.Resource.Type)
	}

	gen := states.CurrentGen
	csrc := changes.GetResourceInstanceChange(addr, gen)
	if csrc == nil {
		log.Printf("[TRACE] EvalReadDiff: No planned change recorded for %s", n.Addr)
		return nil, nil
	}

	change, err := csrc.Decode(schema.ImpliedType())
	if err != nil {
		return nil, fmt.Errorf("failed to decode planned changes for %s: %s", n.Addr, err)
	}

	log.Printf("[TRACE] EvalReadDiff: Read %s change from plan for %s", change.Action, n.Addr)

	return change, nil
}

func (n *NodeAbstractResourceInstance) checkPreventDestroy(change *plans.ResourceInstanceChange) error {
	if change == nil || n.Config == nil || n.Config.Managed == nil {
		return nil
	}

	preventDestroy := n.Config.Managed.PreventDestroy

	if (change.Action == plans.Delete || change.Action.IsReplace()) && preventDestroy {
		var diags tfdiags.Diagnostics
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Instance cannot be destroyed",
			Detail: fmt.Sprintf(
				"Resource %s has lifecycle.prevent_destroy set, but the plan calls for this resource to be destroyed. To avoid this error and continue with the plan, either disable lifecycle.prevent_destroy or reduce the scope of the plan using the -target flag.",
				n.Addr.String(),
			),
			Subject: &n.Config.DeclRange,
		})
		return diags.Err()
	}

	return nil
}

// PreApplyHook calls the pre-Apply hook
func (n *NodeAbstractResourceInstance) PreApplyHook(ctx EvalContext, change *plans.ResourceInstanceChange) tfdiags.Diagnostics {
	var diags tfdiags.Diagnostics

	if change == nil {
		panic(fmt.Sprintf("PreApplyHook for %s called with nil Change", n.Addr))
	}

	if resourceHasUserVisibleApply(n.Addr.Resource) {
		priorState := change.Before
		plannedNewState := change.After

		diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
			return h.PreApply(n.Addr, nil, change.Action, priorState, plannedNewState)
		}))
		if diags.HasErrors() {
			return diags
		}
	}

	return nil
}

// postApplyHook calls the post-Apply hook
func (n *NodeAbstractResourceInstance) postApplyHook(ctx EvalContext, state *states.ResourceInstanceObject, err *error) tfdiags.Diagnostics {
	var diags tfdiags.Diagnostics

	if resourceHasUserVisibleApply(n.Addr.Resource) {
		var newState cty.Value
		if state != nil {
			newState = state.Value
		} else {
			newState = cty.NullVal(cty.DynamicPseudoType)
		}
		diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
			return h.PostApply(n.Addr, nil, newState, *err)
		}))
	}

	diags = diags.Append(*err)

	return diags
}

type phaseState int

const (
	workingState phaseState = iota
	refreshState
)

// writeResourceInstanceState saves the given object as the current object for
// the selected resource instance.
//
// dependencies is a parameter, instead of those directly attacted to the
// NodeAbstractResourceInstance, because we don't write dependencies for
// datasources.
//
// targetState determines which context state we're writing to during plan. The
// default is the global working state.
func (n *NodeAbstractResourceInstance) writeResourceInstanceState(ctx EvalContext, obj *states.ResourceInstanceObject, dependencies []addrs.ConfigResource, targetState phaseState) error {
	absAddr := n.Addr
	_, providerSchema, err := GetProvider(ctx, n.ResolvedProvider)
	if err != nil {
		return err
	}

	var state *states.SyncState
	switch targetState {
	case refreshState:
		log.Printf("[TRACE] writeResourceInstanceState: using RefreshState for %s", absAddr)
		state = ctx.RefreshState()
	default:
		state = ctx.State()
	}

	if obj == nil || obj.Value.IsNull() {
		// No need to encode anything: we'll just write it directly.
		state.SetResourceInstanceCurrent(absAddr, nil, n.ResolvedProvider)
		log.Printf("[TRACE] writeResourceInstanceState: removing state object for %s", absAddr)
		return nil
	}

	// store the new deps in the state.
	// We check for nil here because don't want to override existing dependencies on orphaned nodes.
	if dependencies != nil {
		obj.Dependencies = dependencies
	}

	if providerSchema == nil {
		// Should never happen, unless our state object is nil
		panic("writeResourceInstanceState used with nil ProviderSchema")
	}

	if obj != nil {
		log.Printf("[TRACE] writeResourceInstanceState: writing current state object for %s", absAddr)
	} else {
		log.Printf("[TRACE] writeResourceInstanceState: removing current state object for %s", absAddr)
	}

	schema, currentVersion := (*providerSchema).SchemaForResourceAddr(absAddr.ContainingResource().Resource)
	if schema == nil {
		// It shouldn't be possible to get this far in any real scenario
		// without a schema, but we might end up here in contrived tests that
		// fail to set up their world properly.
		return fmt.Errorf("failed to encode %s in state: no resource type schema available", absAddr)
	}

	src, err := obj.Encode(schema.ImpliedType(), currentVersion)
	if err != nil {
		return fmt.Errorf("failed to encode %s in state: %s", absAddr, err)
	}

	state.SetResourceInstanceCurrent(absAddr, src, n.ResolvedProvider)
	return nil
}

// planDestroy returns a plain destroy diff.
func (n *NodeAbstractResourceInstance) planDestroy(ctx EvalContext, currentState *states.ResourceInstanceObject, deposedKey states.DeposedKey) (*plans.ResourceInstanceChange, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics

	absAddr := n.Addr

	if n.ResolvedProvider.Provider.Type == "" {
		if deposedKey == "" {
			panic(fmt.Sprintf("DestroyPlan for %s does not have ProviderAddr set", absAddr))
		} else {
			panic(fmt.Sprintf("DestroyPlan for %s (deposed %s) does not have ProviderAddr set", absAddr, deposedKey))
		}
	}

	// If there is no state or our attributes object is null then we're already
	// destroyed.
	if currentState == nil || currentState.Value.IsNull() {
		return nil, nil
	}

	// Call pre-diff hook
	diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PreDiff(
			absAddr, deposedKey.Generation(),
			currentState.Value,
			cty.NullVal(cty.DynamicPseudoType),
		)
	}))
	if diags.HasErrors() {
		return nil, diags
	}

	// Plan is always the same for a destroy. We don't need the provider's
	// help for this one.
	plan := &plans.ResourceInstanceChange{
		Addr:       absAddr,
		DeposedKey: deposedKey,
		Change: plans.Change{
			Action: plans.Delete,
			Before: currentState.Value,
			After:  cty.NullVal(cty.DynamicPseudoType),
		},
		Private:      currentState.Private,
		ProviderAddr: n.ResolvedProvider,
	}

	// Call post-diff hook
	diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PostDiff(
			absAddr,
			deposedKey.Generation(),
			plan.Action,
			plan.Before,
			plan.After,
		)
	}))

	return plan, diags
}

// writeChange  saves a planned change for an instance object into the set of
// global planned changes.
func (n *NodeAbstractResourceInstance) writeChange(ctx EvalContext, change *plans.ResourceInstanceChange, deposedKey states.DeposedKey) error {
	changes := ctx.Changes()

	if change == nil {
		// Caller sets nil to indicate that we need to remove a change from
		// the set of changes.
		gen := states.CurrentGen
		if deposedKey != states.NotDeposed {
			gen = deposedKey
		}
		changes.RemoveResourceInstanceChange(n.Addr, gen)
		return nil
	}

	_, providerSchema, err := GetProvider(ctx, n.ResolvedProvider)
	if err != nil {
		return err
	}

	if change.Addr.String() != n.Addr.String() || change.DeposedKey != deposedKey {
		// Should never happen, and indicates a bug in the caller.
		panic("inconsistent address and/or deposed key in WriteChange")
	}

	ri := n.Addr.Resource
	schema, _ := providerSchema.SchemaForResourceAddr(ri.Resource)
	if schema == nil {
		// Should be caught during validation, so we don't bother with a pretty error here
		return fmt.Errorf("provider does not support resource type %q", ri.Resource.Type)
	}

	csrc, err := change.Encode(schema.ImpliedType())
	if err != nil {
		return fmt.Errorf("failed to encode planned changes for %s: %s", n.Addr, err)
	}

	changes.AppendResourceInstanceChange(csrc)
	if deposedKey == states.NotDeposed {
		log.Printf("[TRACE] WriteChange: recorded %s change for %s", change.Action, n.Addr)
	} else {
		log.Printf("[TRACE] WriteChange: recorded %s change for %s deposed object %s", change.Action, n.Addr, deposedKey)
	}

	return nil
}

// refresh does a refresh for a resource
func (n *NodeAbstractResourceInstance) refresh(ctx EvalContext, state *states.ResourceInstanceObject) (*states.ResourceInstanceObject, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics
	absAddr := n.Addr
	provider, providerSchema, err := GetProvider(ctx, n.ResolvedProvider)
	if err != nil {
		return state, diags.Append(err)
	}
	// If we have no state, we don't do any refreshing
	if state == nil {
		log.Printf("[DEBUG] refresh: %s: no state, so not refreshing", absAddr)
		return state, diags
	}

	schema, _ := providerSchema.SchemaForResourceAddr(n.Addr.Resource.ContainingResource())
	if schema == nil {
		// Should be caught during validation, so we don't bother with a pretty error here
		diags = diags.Append(fmt.Errorf("provider does not support resource type %q", n.Addr.Resource.Resource.Type))
		return state, diags
	}

	metaConfigVal := cty.NullVal(cty.DynamicPseudoType)
	if n.ProviderMetas != nil {
		if m, ok := n.ProviderMetas[n.ResolvedProvider.Provider]; ok && m != nil {
			log.Printf("[DEBUG] EvalRefresh: ProviderMeta config value set")
			// if the provider doesn't support this feature, throw an error
			if providerSchema.ProviderMeta == nil {
				log.Printf("[DEBUG] EvalRefresh: no ProviderMeta schema")
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("Provider %s doesn't support provider_meta", n.ResolvedProvider.Provider.String()),
					Detail:   fmt.Sprintf("The resource %s belongs to a provider that doesn't support provider_meta blocks", n.Addr.Resource),
					Subject:  &m.ProviderRange,
				})
			} else {
				log.Printf("[DEBUG] EvalRefresh: ProviderMeta schema found: %+v", providerSchema.ProviderMeta)
				var configDiags tfdiags.Diagnostics
				metaConfigVal, _, configDiags = ctx.EvaluateBlock(m.Config, providerSchema.ProviderMeta, nil, EvalDataForNoInstanceKey)
				diags = diags.Append(configDiags)
				if configDiags.HasErrors() {
					return state, diags
				}
			}
		}
	}

	// Call pre-refresh hook
	diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PreRefresh(absAddr, states.CurrentGen, state.Value)
	}))
	if diags.HasErrors() {
		return state, diags
	}

	// Refresh!
	priorVal := state.Value

	// Unmarked before sending to provider
	var priorPaths []cty.PathValueMarks
	if priorVal.ContainsMarked() {
		priorVal, priorPaths = priorVal.UnmarkDeepWithPaths()
	}

	providerReq := providers.ReadResourceRequest{
		TypeName:     n.Addr.Resource.Resource.Type,
		PriorState:   priorVal,
		Private:      state.Private,
		ProviderMeta: metaConfigVal,
	}

	resp := provider.ReadResource(providerReq)
	diags = diags.Append(resp.Diagnostics)
	if diags.HasErrors() {
		return state, diags
	}

	if resp.NewState == cty.NilVal {
		// This ought not to happen in real cases since it's not possible to
		// send NilVal over the plugin RPC channel, but it can come up in
		// tests due to sloppy mocking.
		panic("new state is cty.NilVal")
	}

	for _, err := range resp.NewState.Type().TestConformance(schema.ImpliedType()) {
		diags = diags.Append(tfdiags.Sourceless(
			tfdiags.Error,
			"Provider produced invalid object",
			fmt.Sprintf(
				"Provider %q planned an invalid value for %s during refresh: %s.\n\nThis is a bug in the provider, which should be reported in the provider's own issue tracker.",
				n.ResolvedProvider.Provider.String(), absAddr, tfdiags.FormatError(err),
			),
		))
	}
	if diags.HasErrors() {
		return state, diags
	}

	// We have no way to exempt provider using the legacy SDK from this check,
	// so we can only log inconsistencies with the updated state values.
	// In most cases these are not errors anyway, and represent "drift" from
	// external changes which will be handled by the subsequent plan.
	if errs := objchange.AssertObjectCompatible(schema, priorVal, resp.NewState); len(errs) > 0 {
		var buf strings.Builder
		fmt.Fprintf(&buf, "[WARN] Provider %q produced an unexpected new value for %s during refresh.", n.ResolvedProvider.Provider.String(), absAddr)
		for _, err := range errs {
			fmt.Fprintf(&buf, "\n      - %s", tfdiags.FormatError(err))
		}
		log.Print(buf.String())
	}

	ret := state.DeepCopy()
	ret.Value = resp.NewState
	ret.Private = resp.Private
	ret.Dependencies = state.Dependencies
	ret.CreateBeforeDestroy = state.CreateBeforeDestroy

	// Call post-refresh hook
	diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PostRefresh(absAddr, states.CurrentGen, priorVal, ret.Value)
	}))
	if diags.HasErrors() {
		return ret, diags
	}

	// Mark the value if necessary
	if len(priorPaths) > 0 {
		ret.Value = ret.Value.MarkWithPaths(priorPaths)
	}

	return ret, diags
}

func (n *NodeAbstractResourceInstance) plan(
	ctx EvalContext,
	plannedChange *plans.ResourceInstanceChange,
	currentState *states.ResourceInstanceObject,
	createBeforeDestroy bool) (*plans.ResourceInstanceChange, *states.ResourceInstanceObject, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics
	var state *states.ResourceInstanceObject
	var plan *plans.ResourceInstanceChange

	config := *n.Config
	resource := n.Addr.Resource.Resource
	provider, providerSchema, err := GetProvider(ctx, n.ResolvedProvider)
	if err != nil {
		return plan, state, diags.Append(err)
	}

	if plannedChange != nil {
		// If we already planned the action, we stick to that plan
		createBeforeDestroy = plannedChange.Action == plans.CreateThenDelete
	}

	if providerSchema == nil {
		diags = diags.Append(fmt.Errorf("provider schema is unavailable for %s", n.Addr))
		return plan, state, diags
	}

	// Evaluate the configuration
	schema, _ := providerSchema.SchemaForResourceAddr(resource)
	if schema == nil {
		// Should be caught during validation, so we don't bother with a pretty error here
		diags = diags.Append(fmt.Errorf("provider does not support resource type %q", resource.Type))
		return plan, state, diags
	}

	forEach, _ := evaluateForEachExpression(n.Config.ForEach, ctx)

	keyData := EvalDataForInstanceKey(n.ResourceInstanceAddr().Resource.Key, forEach)
	origConfigVal, _, configDiags := ctx.EvaluateBlock(config.Config, schema, nil, keyData)
	diags = diags.Append(configDiags)
	if configDiags.HasErrors() {
		return plan, state, diags
	}

	metaConfigVal := cty.NullVal(cty.DynamicPseudoType)
	if n.ProviderMetas != nil {
		if m, ok := n.ProviderMetas[n.ResolvedProvider.Provider]; ok && m != nil {
			// if the provider doesn't support this feature, throw an error
			if providerSchema.ProviderMeta == nil {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("Provider %s doesn't support provider_meta", n.ResolvedProvider.Provider),
					Detail:   fmt.Sprintf("The resource %s belongs to a provider that doesn't support provider_meta blocks", n.Addr.Resource),
					Subject:  &m.ProviderRange,
				})
			} else {
				var configDiags tfdiags.Diagnostics
				metaConfigVal, _, configDiags = ctx.EvaluateBlock(m.Config, providerSchema.ProviderMeta, nil, EvalDataForNoInstanceKey)
				diags = diags.Append(configDiags)
				if configDiags.HasErrors() {
					return plan, state, diags
				}
			}
		}
	}

	var priorVal cty.Value
	var priorValTainted cty.Value
	var priorPrivate []byte
	if currentState != nil {
		if currentState.Status != states.ObjectTainted {
			priorVal = currentState.Value
			priorPrivate = currentState.Private
		} else {
			// If the prior state is tainted then we'll proceed below like
			// we're creating an entirely new object, but then turn it into
			// a synthetic "Replace" change at the end, creating the same
			// result as if the provider had marked at least one argument
			// change as "requires replacement".
			priorValTainted = currentState.Value
			priorVal = cty.NullVal(schema.ImpliedType())
		}
	} else {
		priorVal = cty.NullVal(schema.ImpliedType())
	}

	// Create an unmarked version of our config val and our prior val.
	// Store the paths for the config val to re-markafter
	// we've sent things over the wire.
	unmarkedConfigVal, unmarkedPaths := origConfigVal.UnmarkDeepWithPaths()
	unmarkedPriorVal, priorPaths := priorVal.UnmarkDeepWithPaths()

	log.Printf("[TRACE] Re-validating config for %q", n.Addr)
	// Allow the provider to validate the final set of values.
	// The config was statically validated early on, but there may have been
	// unknown values which the provider could not validate at the time.
	// TODO: It would be more correct to validate the config after
	// ignore_changes has been applied, but the current implementation cannot
	// exclude computed-only attributes when given the `all` option.
	validateResp := provider.ValidateResourceTypeConfig(
		providers.ValidateResourceTypeConfigRequest{
			TypeName: n.Addr.Resource.Resource.Type,
			Config:   unmarkedConfigVal,
		},
	)
	if validateResp.Diagnostics.HasErrors() {
		diags = diags.Append(validateResp.Diagnostics.InConfigBody(config.Config))
		return plan, state, diags
	}

	// ignore_changes is meant to only apply to the configuration, so it must
	// be applied before we generate a plan. This ensures the config used for
	// the proposed value, the proposed value itself, and the config presented
	// to the provider in the PlanResourceChange request all agree on the
	// starting values.
	configValIgnored, ignoreChangeDiags := n.processIgnoreChanges(unmarkedPriorVal, unmarkedConfigVal)
	diags = diags.Append(ignoreChangeDiags)
	if ignoreChangeDiags.HasErrors() {
		return plan, state, diags
	}

	proposedNewVal := objchange.ProposedNewObject(schema, unmarkedPriorVal, configValIgnored)

	// Call pre-diff hook
	diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PreDiff(n.Addr, states.CurrentGen, priorVal, proposedNewVal)
	}))
	if diags.HasErrors() {
		return plan, state, diags
	}

	resp := provider.PlanResourceChange(providers.PlanResourceChangeRequest{
		TypeName:         n.Addr.Resource.Resource.Type,
		Config:           configValIgnored,
		PriorState:       unmarkedPriorVal,
		ProposedNewState: proposedNewVal,
		PriorPrivate:     priorPrivate,
		ProviderMeta:     metaConfigVal,
	})
	diags = diags.Append(resp.Diagnostics.InConfigBody(config.Config))
	if diags.HasErrors() {
		return plan, state, diags
	}

	plannedNewVal := resp.PlannedState
	plannedPrivate := resp.PlannedPrivate

	if plannedNewVal == cty.NilVal {
		// Should never happen. Since real-world providers return via RPC a nil
		// is always a bug in the client-side stub. This is more likely caused
		// by an incompletely-configured mock provider in tests, though.
		panic(fmt.Sprintf("PlanResourceChange of %s produced nil value", n.Addr))
	}

	// We allow the planned new value to disagree with configuration _values_
	// here, since that allows the provider to do special logic like a
	// DiffSuppressFunc, but we still require that the provider produces
	// a value whose type conforms to the schema.
	for _, err := range plannedNewVal.Type().TestConformance(schema.ImpliedType()) {
		diags = diags.Append(tfdiags.Sourceless(
			tfdiags.Error,
			"Provider produced invalid plan",
			fmt.Sprintf(
				"Provider %q planned an invalid value for %s.\n\nThis is a bug in the provider, which should be reported in the provider's own issue tracker.",
				n.ResolvedProvider.Provider, tfdiags.FormatErrorPrefixed(err, n.Addr.String()),
			),
		))
	}
	if diags.HasErrors() {
		return plan, state, diags
	}

	if errs := objchange.AssertPlanValid(schema, unmarkedPriorVal, configValIgnored, plannedNewVal); len(errs) > 0 {
		if resp.LegacyTypeSystem {
			// The shimming of the old type system in the legacy SDK is not precise
			// enough to pass this consistency check, so we'll give it a pass here,
			// but we will generate a warning about it so that we are more likely
			// to notice in the logs if an inconsistency beyond the type system
			// leads to a downstream provider failure.
			var buf strings.Builder
			fmt.Fprintf(&buf,
				"[WARN] Provider %q produced an invalid plan for %s, but we are tolerating it because it is using the legacy plugin SDK.\n    The following problems may be the cause of any confusing errors from downstream operations:",
				n.ResolvedProvider.Provider, n.Addr,
			)
			for _, err := range errs {
				fmt.Fprintf(&buf, "\n      - %s", tfdiags.FormatError(err))
			}
			log.Print(buf.String())
		} else {
			for _, err := range errs {
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Error,
					"Provider produced invalid plan",
					fmt.Sprintf(
						"Provider %q planned an invalid value for %s.\n\nThis is a bug in the provider, which should be reported in the provider's own issue tracker.",
						n.ResolvedProvider.Provider, tfdiags.FormatErrorPrefixed(err, n.Addr.String()),
					),
				))
			}
			return plan, state, diags
		}
	}

	if resp.LegacyTypeSystem {
		// Because we allow legacy providers to depart from the contract and
		// return changes to non-computed values, the plan response may have
		// altered values that were already suppressed with ignore_changes.
		// A prime example of this is where providers attempt to obfuscate
		// config data by turning the config value into a hash and storing the
		// hash value in the state. There are enough cases of this in existing
		// providers that we must accommodate the behavior for now, so for
		// ignore_changes to work at all on these values, we will revert the
		// ignored values once more.
		plannedNewVal, ignoreChangeDiags = n.processIgnoreChanges(unmarkedPriorVal, plannedNewVal)
		diags = diags.Append(ignoreChangeDiags)
		if ignoreChangeDiags.HasErrors() {
			return plan, state, diags
		}
	}

	// Add the marks back to the planned new value -- this must happen after ignore changes
	// have been processed
	unmarkedPlannedNewVal := plannedNewVal
	if len(unmarkedPaths) > 0 {
		plannedNewVal = plannedNewVal.MarkWithPaths(unmarkedPaths)
	}

	// The provider produces a list of paths to attributes whose changes mean
	// that we must replace rather than update an existing remote object.
	// However, we only need to do that if the identified attributes _have_
	// actually changed -- particularly after we may have undone some of the
	// changes in processIgnoreChanges -- so now we'll filter that list to
	// include only where changes are detected.
	reqRep := cty.NewPathSet()
	if len(resp.RequiresReplace) > 0 {
		for _, path := range resp.RequiresReplace {
			if priorVal.IsNull() {
				// If prior is null then we don't expect any RequiresReplace at all,
				// because this is a Create action.
				continue
			}

			priorChangedVal, priorPathDiags := hcl.ApplyPath(unmarkedPriorVal, path, nil)
			plannedChangedVal, plannedPathDiags := hcl.ApplyPath(plannedNewVal, path, nil)
			if plannedPathDiags.HasErrors() && priorPathDiags.HasErrors() {
				// This means the path was invalid in both the prior and new
				// values, which is an error with the provider itself.
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Error,
					"Provider produced invalid plan",
					fmt.Sprintf(
						"Provider %q has indicated \"requires replacement\" on %s for a non-existent attribute path %#v.\n\nThis is a bug in the provider, which should be reported in the provider's own issue tracker.",
						n.ResolvedProvider.Provider, n.Addr, path,
					),
				))
				continue
			}

			// Make sure we have valid Values for both values.
			// Note: if the opposing value was of the type
			// cty.DynamicPseudoType, the type assigned here may not exactly
			// match the schema. This is fine here, since we're only going to
			// check for equality, but if the NullVal is to be used, we need to
			// check the schema for th true type.
			switch {
			case priorChangedVal == cty.NilVal && plannedChangedVal == cty.NilVal:
				// this should never happen without ApplyPath errors above
				panic("requires replace path returned 2 nil values")
			case priorChangedVal == cty.NilVal:
				priorChangedVal = cty.NullVal(plannedChangedVal.Type())
			case plannedChangedVal == cty.NilVal:
				plannedChangedVal = cty.NullVal(priorChangedVal.Type())
			}

			// Unmark for this value for the equality test. If only sensitivity has changed,
			// this does not require an Update or Replace
			unmarkedPlannedChangedVal, _ := plannedChangedVal.UnmarkDeep()
			eqV := unmarkedPlannedChangedVal.Equals(priorChangedVal)
			if !eqV.IsKnown() || eqV.False() {
				reqRep.Add(path)
			}
		}
		if diags.HasErrors() {
			return plan, state, diags
		}
	}

	// Unmark for this test for value equality.
	eqV := unmarkedPlannedNewVal.Equals(unmarkedPriorVal)
	eq := eqV.IsKnown() && eqV.True()

	var action plans.Action
	switch {
	case priorVal.IsNull():
		action = plans.Create
	case eq:
		action = plans.NoOp
	case !reqRep.Empty():
		// If there are any "requires replace" paths left _after our filtering
		// above_ then this is a replace action.
		if createBeforeDestroy {
			action = plans.CreateThenDelete
		} else {
			action = plans.DeleteThenCreate
		}
	default:
		action = plans.Update
		// "Delete" is never chosen here, because deletion plans are always
		// created more directly elsewhere, such as in "orphan" handling.
	}

	if action.IsReplace() {
		// In this strange situation we want to produce a change object that
		// shows our real prior object but has a _new_ object that is built
		// from a null prior object, since we're going to delete the one
		// that has all the computed values on it.
		//
		// Therefore we'll ask the provider to plan again here, giving it
		// a null object for the prior, and then we'll meld that with the
		// _actual_ prior state to produce a correctly-shaped replace change.
		// The resulting change should show any computed attributes changing
		// from known prior values to unknown values, unless the provider is
		// able to predict new values for any of these computed attributes.
		nullPriorVal := cty.NullVal(schema.ImpliedType())

		// Since there is no prior state to compare after replacement, we need
		// a new unmarked config from our original with no ignored values.
		unmarkedConfigVal := origConfigVal
		if origConfigVal.ContainsMarked() {
			unmarkedConfigVal, _ = origConfigVal.UnmarkDeep()
		}

		// create a new proposed value from the null state and the config
		proposedNewVal = objchange.ProposedNewObject(schema, nullPriorVal, unmarkedConfigVal)

		resp = provider.PlanResourceChange(providers.PlanResourceChangeRequest{
			TypeName:         n.Addr.Resource.Resource.Type,
			Config:           unmarkedConfigVal,
			PriorState:       nullPriorVal,
			ProposedNewState: proposedNewVal,
			PriorPrivate:     plannedPrivate,
			ProviderMeta:     metaConfigVal,
		})
		// We need to tread carefully here, since if there are any warnings
		// in here they probably also came out of our previous call to
		// PlanResourceChange above, and so we don't want to repeat them.
		// Consequently, we break from the usual pattern here and only
		// append these new diagnostics if there's at least one error inside.
		if resp.Diagnostics.HasErrors() {
			diags = diags.Append(resp.Diagnostics.InConfigBody(config.Config))
			return plan, state, diags
		}
		plannedNewVal = resp.PlannedState
		plannedPrivate = resp.PlannedPrivate

		if len(unmarkedPaths) > 0 {
			plannedNewVal = plannedNewVal.MarkWithPaths(unmarkedPaths)
		}

		for _, err := range plannedNewVal.Type().TestConformance(schema.ImpliedType()) {
			diags = diags.Append(tfdiags.Sourceless(
				tfdiags.Error,
				"Provider produced invalid plan",
				fmt.Sprintf(
					"Provider %q planned an invalid value for %s%s.\n\nThis is a bug in the provider, which should be reported in the provider's own issue tracker.",
					n.ResolvedProvider.Provider, n.Addr, tfdiags.FormatError(err),
				),
			))
		}
		if diags.HasErrors() {
			return plan, state, diags
		}
	}

	// If our prior value was tainted then we actually want this to appear
	// as a replace change, even though so far we've been treating it as a
	// create.
	if action == plans.Create && priorValTainted != cty.NilVal {
		if createBeforeDestroy {
			action = plans.CreateThenDelete
		} else {
			action = plans.DeleteThenCreate
		}
		priorVal = priorValTainted
	}

	// If we plan to write or delete sensitive paths from state,
	// this is an Update action
	if action == plans.NoOp && !reflect.DeepEqual(priorPaths, unmarkedPaths) {
		action = plans.Update
	}

	// As a special case, if we have a previous diff (presumably from the plan
	// phases, whereas we're now in the apply phase) and it was for a replace,
	// we've already deleted the original object from state by the time we
	// get here and so we would've ended up with a _create_ action this time,
	// which we now need to paper over to get a result consistent with what
	// we originally intended.
	if plannedChange != nil {
		prevChange := *plannedChange
		if prevChange.Action.IsReplace() && action == plans.Create {
			log.Printf("[TRACE] EvalDiff: %s treating Create change as %s change to match with earlier plan", n.Addr, prevChange.Action)
			action = prevChange.Action
			priorVal = prevChange.Before
		}
	}

	// Call post-refresh hook
	diags = diags.Append(ctx.Hook(func(h Hook) (HookAction, error) {
		return h.PostDiff(n.Addr, states.CurrentGen, action, priorVal, plannedNewVal)
	}))
	if diags.HasErrors() {
		return plan, state, diags
	}

	// Update our return plan
	plan = &plans.ResourceInstanceChange{
		Addr:         n.Addr,
		Private:      plannedPrivate,
		ProviderAddr: n.ResolvedProvider,
		Change: plans.Change{
			Action: action,
			Before: priorVal,
			// Pass the marked planned value through in our change
			// to propogate through evaluation.
			// Marks will be removed when encoding.
			After: plannedNewVal,
		},
		RequiredReplace: reqRep,
	}

	// Update our return state
	state = &states.ResourceInstanceObject{
		// We use the special "planned" status here to note that this
		// object's value is not yet complete. Objects with this status
		// cannot be used during expression evaluation, so the caller
		// must _also_ record the returned change in the active plan,
		// which the expression evaluator will use in preference to this
		// incomplete value recorded in the state.
		Status:  states.ObjectPlanned,
		Value:   plannedNewVal,
		Private: plannedPrivate,
	}

	return plan, state, diags
}

func (n *NodeAbstractResource) processIgnoreChanges(prior, config cty.Value) (cty.Value, tfdiags.Diagnostics) {
	// ignore_changes only applies when an object already exists, since we
	// can't ignore changes to a thing we've not created yet.
	if prior.IsNull() {
		return config, nil
	}

	ignoreChanges := n.Config.Managed.IgnoreChanges
	ignoreAll := n.Config.Managed.IgnoreAllChanges

	if len(ignoreChanges) == 0 && !ignoreAll {
		return config, nil
	}
	if ignoreAll {
		return prior, nil
	}
	if prior.IsNull() || config.IsNull() {
		// Ignore changes doesn't apply when we're creating for the first time.
		// Proposed should never be null here, but if it is then we'll just let it be.
		return config, nil
	}

	return processIgnoreChangesIndividual(prior, config, ignoreChanges)
}

func processIgnoreChangesIndividual(prior, config cty.Value, ignoreChanges []hcl.Traversal) (cty.Value, tfdiags.Diagnostics) {
	// When we walk below we will be using cty.Path values for comparison, so
	// we'll convert our traversals here so we can compare more easily.
	ignoreChangesPath := make([]cty.Path, len(ignoreChanges))
	for i, traversal := range ignoreChanges {
		path := make(cty.Path, len(traversal))
		for si, step := range traversal {
			switch ts := step.(type) {
			case hcl.TraverseRoot:
				path[si] = cty.GetAttrStep{
					Name: ts.Name,
				}
			case hcl.TraverseAttr:
				path[si] = cty.GetAttrStep{
					Name: ts.Name,
				}
			case hcl.TraverseIndex:
				path[si] = cty.IndexStep{
					Key: ts.Key,
				}
			default:
				panic(fmt.Sprintf("unsupported traversal step %#v", step))
			}
		}
		ignoreChangesPath[i] = path
	}

	type ignoreChange struct {
		// Path is the full path, minus any trailing map index
		path cty.Path
		// Value is the value we are to retain at the above path. If there is a
		// key value, this must be a map and the desired value will be at the
		// key index.
		value cty.Value
		// Key is the index key if the ignored path ends in a map index.
		key cty.Value
	}
	var ignoredValues []ignoreChange

	// Find the actual changes first and store them in the ignoreChange struct.
	// If the change was to a map value, and the key doesn't exist in the
	// config, it would never be visited in the transform walk.
	for _, icPath := range ignoreChangesPath {
		key := cty.NullVal(cty.String)
		// check for a map index, since maps are the only structure where we
		// could have invalid path steps.
		last, ok := icPath[len(icPath)-1].(cty.IndexStep)
		if ok {
			if last.Key.Type() == cty.String {
				icPath = icPath[:len(icPath)-1]
				key = last.Key
			}
		}

		// The structure should have been validated already, and we already
		// trimmed the trailing map index. Any other intermediate index error
		// means we wouldn't be able to apply the value below, so no need to
		// record this.
		p, err := icPath.Apply(prior)
		if err != nil {
			continue
		}
		c, err := icPath.Apply(config)
		if err != nil {
			continue
		}

		// If this is a map, it is checking the entire map value for equality
		// rather than the individual key. This means that the change is stored
		// here even if our ignored key doesn't change. That is OK since it
		// won't cause any changes in the transformation, but allows us to skip
		// breaking up the maps and checking for key existence here too.
		eq := p.Equals(c)
		if eq.IsKnown() && eq.False() {
			// there a change to ignore at this path, store the prior value
			ignoredValues = append(ignoredValues, ignoreChange{icPath, p, key})
		}
	}

	if len(ignoredValues) == 0 {
		return config, nil
	}

	ret, _ := cty.Transform(config, func(path cty.Path, v cty.Value) (cty.Value, error) {
		// Easy path for when we are only matching the entire value. The only
		// values we break up for inspection are maps.
		if !v.Type().IsMapType() {
			for _, ignored := range ignoredValues {
				if path.Equals(ignored.path) {
					return ignored.value, nil
				}
			}
			return v, nil
		}
		// We now know this must be a map, so we need to accumulate the values
		// key-by-key.

		if !v.IsNull() && !v.IsKnown() {
			// since v is not known, we cannot ignore individual keys
			return v, nil
		}

		// The configMap is the current configuration value, which we will
		// mutate based on the ignored paths and the prior map value.
		var configMap map[string]cty.Value
		switch {
		case v.IsNull() || v.LengthInt() == 0:
			configMap = map[string]cty.Value{}
		default:
			configMap = v.AsValueMap()
		}

		for _, ignored := range ignoredValues {
			if !path.Equals(ignored.path) {
				continue
			}

			if ignored.key.IsNull() {
				// The map address is confirmed to match at this point,
				// so if there is no key, we want the entire map and can
				// stop accumulating values.
				return ignored.value, nil
			}
			// Now we know we are ignoring a specific index of this map, so get
			// the config map and modify, add, or remove the desired key.

			// We also need to create a prior map, so we can check for
			// existence while getting the value, because Value.Index will
			// return null for a key with a null value and for a non-existent
			// key.
			var priorMap map[string]cty.Value
			switch {
			case ignored.value.IsNull() || ignored.value.LengthInt() == 0:
				priorMap = map[string]cty.Value{}
			default:
				priorMap = ignored.value.AsValueMap()
			}

			key := ignored.key.AsString()
			priorElem, keep := priorMap[key]

			switch {
			case !keep:
				// this didn't exist in the old map value, so we're keeping the
				// "absence" of the key by removing it from the config
				delete(configMap, key)
			default:
				configMap[key] = priorElem
			}
		}

		if len(configMap) == 0 {
			return cty.MapValEmpty(v.Type().ElementType()), nil
		}

		return cty.MapVal(configMap), nil
	})
	return ret, nil
}
