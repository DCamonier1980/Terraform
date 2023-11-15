// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tfstackdata1

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"

	"github.com/hashicorp/terraform/internal/addrs"
	"github.com/hashicorp/terraform/internal/plans"
	"github.com/hashicorp/terraform/internal/plans/planproto"
	"github.com/hashicorp/terraform/internal/rpcapi/terraform1"
	"github.com/hashicorp/terraform/internal/states"
)

func ResourceInstanceObjectStateToTFStackData1(objSrc *states.ResourceInstanceObjectSrc, providerConfigAddr addrs.AbsProviderConfig) *StateResourceInstanceObjectV1 {
	if objSrc == nil {
		// This is presumably representing the absense of any prior state,
		// such as when an object is being planned for creation.
		return nil
	}

	// Hack: we'll borrow NewDynamicValue's treatment of the sensitive
	// attribute paths here just so we don't need to reimplement the
	// slice-of-paths conversion in yet another place. We don't
	// actually do anything with the value part of this.
	protoValue := terraform1.NewDynamicValue(plans.DynamicValue(nil), objSrc.AttrSensitivePaths)
	rawMsg := &StateResourceInstanceObjectV1{
		SchemaVersion:        objSrc.SchemaVersion,
		ValueJson:            objSrc.AttrsJSON,
		SensitivePaths:       Terraform1ToPlanProtoAttributePaths(protoValue.Sensitive),
		CreateBeforeDestroy:  objSrc.CreateBeforeDestroy,
		ProviderConfigAddr:   providerConfigAddr.String(),
		ProviderSpecificData: objSrc.Private,
	}
	switch objSrc.Status {
	case states.ObjectReady:
		rawMsg.Status = StateResourceInstanceObjectV1_READY
	case states.ObjectTainted:
		rawMsg.Status = StateResourceInstanceObjectV1_DAMAGED
	default:
		rawMsg.Status = StateResourceInstanceObjectV1_UNKNOWN
	}

	rawMsg.Dependencies = make([]string, len(objSrc.Dependencies))
	for i, addr := range objSrc.Dependencies {
		rawMsg.Dependencies[i] = addr.String()
	}

	return rawMsg
}

func Terraform1ToPlanProtoAttributePaths(paths []*terraform1.AttributePath) []*planproto.Path {
	if len(paths) == 0 {
		return nil
	}
	ret := make([]*planproto.Path, len(paths))
	for i, tf1Path := range paths {
		ret[i] = Terraform1ToPlanProtoAttributePath(tf1Path)
	}
	return ret
}

func Terraform1ToPlanProtoAttributePath(path *terraform1.AttributePath) *planproto.Path {
	if path == nil {
		return nil
	}
	ret := &planproto.Path{}
	if len(path.Steps) == 0 {
		return ret
	}
	ret.Steps = make([]*planproto.Path_Step, len(path.Steps))
	for i, tf1Step := range path.Steps {
		ret.Steps[i] = Terraform1ToPlanProtoAttributePathStep(tf1Step)
	}
	return ret
}

func Terraform1ToPlanProtoAttributePathStep(step *terraform1.AttributePath_Step) *planproto.Path_Step {
	if step == nil {
		return nil
	}
	ret := &planproto.Path_Step{}
	switch sel := step.Selector.(type) {
	case *terraform1.AttributePath_Step_AttributeName:
		ret.Selector = &planproto.Path_Step_AttributeName{
			AttributeName: sel.AttributeName,
		}
	case *terraform1.AttributePath_Step_ElementKeyInt:
		encInt, err := msgpack.Marshal(cty.NumberIntVal(sel.ElementKeyInt), cty.Number)
		if err != nil {
			// This should not be possible because all integers have a cty msgpack encoding
			panic(fmt.Sprintf("unencodable element index: %s", err))
		}
		ret.Selector = &planproto.Path_Step_ElementKey{
			ElementKey: &planproto.DynamicValue{
				Msgpack: encInt,
			},
		}
	case *terraform1.AttributePath_Step_ElementKeyString:
		encStr, err := msgpack.Marshal(cty.StringVal(sel.ElementKeyString), cty.String)
		if err != nil {
			// This should not be possible because all strings have a cty msgpack encoding
			panic(fmt.Sprintf("unencodable element key: %s", err))
		}
		ret.Selector = &planproto.Path_Step_ElementKey{
			ElementKey: &planproto.DynamicValue{
				Msgpack: encStr,
			},
		}
	default:
		// Should not get here, because the above cases should be exhaustive
		// for all possible *terraform1.AttributePath_Step selector types.
		panic(fmt.Sprintf("unsupported path step selector type %T", sel))
	}
	return ret
}
