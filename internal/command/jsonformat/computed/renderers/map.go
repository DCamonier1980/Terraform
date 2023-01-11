package renderers

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform/internal/command/jsonformat/computed"

	"github.com/hashicorp/terraform/internal/command/format"
	"github.com/hashicorp/terraform/internal/plans"
)

var _ computed.DiffRenderer = (*mapRenderer)(nil)

func Map(elements map[string]computed.Diff) computed.DiffRenderer {
	return &mapRenderer{
		elements: elements,
	}
}

func NestedMap(elements map[string]computed.Diff) computed.DiffRenderer {
	return &mapRenderer{
		elements:                  elements,
		overrideNullSuffix:        true,
		overrideForcesReplacement: true,
	}
}

type mapRenderer struct {
	NoWarningsRenderer

	elements map[string]computed.Diff

	overrideNullSuffix        bool
	overrideForcesReplacement bool
}

func (renderer mapRenderer) RenderHuman(diff computed.Diff, indent int, opts computed.RenderHumanOpts) string {
	forcesReplacementSelf := diff.Replace && !renderer.overrideForcesReplacement
	forcesReplacementChildren := diff.Replace && renderer.overrideForcesReplacement

	if len(renderer.elements) == 0 {
		return fmt.Sprintf("{}%s%s", nullSuffix(opts.OverrideNullSuffix, diff.Action), forcesReplacement(forcesReplacementSelf, opts.OverrideForcesReplacement))
	}

	maximumKeyLen := 0
	for key := range renderer.elements {
		if maximumKeyLen < len(key) {
			maximumKeyLen = len(key)
		}
	}
	maximumKeyLen += 2 // We always render map keys with quotation marks.

	unchangedElements := 0

	// Sort the map elements by key, so we have a deterministic ordering in
	// the output.
	var keys []string
	for key := range renderer.elements {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	elementOpts := opts.Clone()
	elementOpts.OverrideNullSuffix = diff.Action == plans.Delete || renderer.overrideNullSuffix
	elementOpts.OverrideForcesReplacement = forcesReplacementChildren

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("{%s\n", forcesReplacement(forcesReplacementSelf, opts.OverrideForcesReplacement)))
	for _, key := range keys {
		element := renderer.elements[key]

		if element.Action == plans.NoOp && !opts.ShowUnchangedChildren {
			// Don't render NoOp operations when we are compact display.
			unchangedElements++
			continue
		}

		for _, warning := range element.WarningsHuman(indent + 1) {
			buf.WriteString(fmt.Sprintf("%s%s\n", formatIndent(indent+1), warning))
		}

		// Only show commas between elements for objects.
		comma := ""
		if _, ok := element.Renderer.(*objectRenderer); ok {
			comma = ","
		}

		buf.WriteString(fmt.Sprintf("%s%s %-*q = %s%s\n", formatIndent(indent+1), format.DiffActionSymbol(element.Action), maximumKeyLen, key, element.RenderHuman(indent+1, elementOpts), comma))
	}

	if unchangedElements > 0 {
		buf.WriteString(fmt.Sprintf("%s%s %s\n", formatIndent(indent+1), format.DiffActionSymbol(plans.NoOp), unchanged("element", unchangedElements)))
	}

	buf.WriteString(fmt.Sprintf("%s%s }%s", formatIndent(indent), format.DiffActionSymbol(plans.NoOp), nullSuffix(opts.OverrideNullSuffix, diff.Action)))
	return buf.String()
}
