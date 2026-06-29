package ir

import (
	"fmt"
	"strings"
)

// CFGToText returns a textual dump of the CFG rooted at entry, suitable for
// debugging and golden-file diffs. Blocks are numbered by their position in
// reverse-postorder traversal (0, 1, 2, ...).
func CFGToText(entry *BasicBlock) string {
	blocks := traverseReversePostorder(entry)
	idx := make(map[*BasicBlock]int, len(blocks))
	for i, b := range blocks {
		idx[b] = i
	}
	var sb strings.Builder
	for i, block := range blocks {
		fmt.Fprintf(&sb, "block %d (%d in):\n", i, len(block.Incoming))
		for _, phi := range block.Phis {
			fmt.Fprintf(&sb, "  phi %s =", FormatPlace(phi.Target))
			for pred, arg := range phi.Args {
				fmt.Fprintf(&sb, " [B%d→%s]", idx[pred], FormatPlace(arg))
			}
			sb.WriteByte('\n')
		}
		for _, stmt := range block.Statements {
			fmt.Fprintf(&sb, "  %s\n", FormatNode(stmt))
		}
		if block.Test != nil {
			fmt.Fprintf(&sb, "  test %s\n", FormatNode(block.Test))
		}
		for _, out := range block.Outgoing {
			targetID := -1
			if out.Dst != nil {
				targetID = idx[out.Dst]
			}
			if out.Cond == nil {
				fmt.Fprintf(&sb, "  -> block %d (uncond)\n", targetID)
			} else {
				fmt.Fprintf(&sb, "  -> block %d (cond=%.0f)\n", targetID, *out.Cond)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// CFGToMermaid returns a Mermaid.js flowchart description of the CFG rooted at
// entry. The output can be rendered at https://mermaid.live.
func CFGToMermaid(entry *BasicBlock) string {
	blocks := traverseReversePostorder(entry)
	idx := make(map[*BasicBlock]int, len(blocks))
	for i, b := range blocks {
		idx[b] = i
	}
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")
	for i, block := range blocks {
		label := fmt.Sprintf("B%d<br/>", i)
		for _, phi := range block.Phis {
			label += fmt.Sprintf("φ %s<br/>", FormatPlace(phi.Target))
		}
		for _, stmt := range block.Statements {
			label += fmt.Sprintf("%s<br/>", FormatNode(stmt))
		}
		if block.Test != nil {
			label += fmt.Sprintf("<i>test</i> %s", FormatNode(block.Test))
		}
		label = strings.ReplaceAll(label, `"`, `'`)
		fmt.Fprintf(&sb, "  B%d[\"%s\"]\n", i, label)
	}
	for _, block := range blocks {
		for _, out := range block.Outgoing {
			edgeLabel := ""
			if out.Cond != nil {
				edgeLabel = fmt.Sprintf("|%.0f|", *out.Cond)
			}
			targetName := "EXIT"
			if out.Dst != nil {
				targetName = fmt.Sprintf("B%d", idx[out.Dst])
			}
			fmt.Fprintf(&sb, "  B%d -->%s %s\n", idx[block], edgeLabel, targetName)
		}
	}
	return sb.String()
}

// FormatNode returns a human-readable representation of a single IR node.
func FormatNode(n Node) string {
	if n == nil {
		return "nil"
	}
	switch t := n.(type) {
	case Const:
		return fmt.Sprintf("Const(%.6g)", float64(t))
	case Instr:
		args := make([]string, len(t.Args))
		for i, arg := range t.Args {
			args[i] = FormatNode(arg)
		}
		return fmt.Sprintf("%s(%s)", string(t.Op), strings.Join(args, ", "))
	case Get:
		return fmt.Sprintf("Get(%s)", FormatPlace(t.Place))
	case Set:
		return fmt.Sprintf("Set(%s, %s)", FormatPlace(t.Place), FormatNode(t.Value))
	default:
		return fmt.Sprintf("%T", n)
	}
}

// FormatPlace returns a human-readable representation of a Place.
func FormatPlace(p Place) string {
	switch t := p.(type) {
	case BlockPlace:
		return fmt.Sprintf("[%s+%s+#%d]", FormatNode(t.Block), FormatNode(t.Index), t.Offset)
	case SSAPlace:
		return fmt.Sprintf("%s.%d", t.Name, t.Num)
	default:
		return fmt.Sprintf("%T", p)
	}
}
