package differ

import (
	"github.com/hashicorp/terraform/internal/command/jsonformat/change"
	"github.com/hashicorp/terraform/internal/command/jsonprovider"
	"github.com/hashicorp/terraform/internal/plans"
)

func (v Value) computeChangeForBlock(block *jsonprovider.Block) change.Change {
	current := v.getDefaultActionForIteration()

	blockValue := v.asMap()

	attributes := make(map[string]change.Change)
	for key, attr := range block.Attributes {
		childValue := blockValue.getChild(key)
		childChange := childValue.ComputeChange(attr)
		if childChange.Action() == plans.NoOp && childValue.Before == nil && childValue.After == nil {
			// Don't record nil values at all in blocks.
			continue
		}

		attributes[key] = childChange
		current = compareActions(current, childChange.Action())
	}

	blocks := make(map[string][]change.Change)
	for key, blockType := range block.BlockTypes {
		childValue := blockValue.getChild(key)
		childChanges, next := childValue.computeChangesForBlockType(blockType)
		if next == plans.NoOp && childValue.Before == nil && childValue.After == nil {
			// Don't record nil values at all in blocks.
			continue
		}
		blocks[key] = childChanges
		current = compareActions(current, next)
	}

	return change.New(change.Block(attributes, blocks), current, v.replacePath())
}

func (v Value) computeChangesForBlockType(blockType *jsonprovider.BlockType) ([]change.Change, plans.Action) {
	switch NestingMode(blockType.NestingMode) {
	case nestingModeSet:
		return v.computeBlockChangesAsSet(blockType.Block)
	case nestingModeList:
		return v.computeBlockChangesAsList(blockType.Block)
	case nestingModeMap:
		return v.computeBlockChangesAsMap(blockType.Block)
	case nestingModeSingle, nestingModeGroup:
		ch := v.ComputeChange(blockType.Block)
		return []change.Change{ch}, ch.Action()
	default:
		panic("unrecognized nesting mode: " + blockType.NestingMode)
	}
}
