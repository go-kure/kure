package admission

import (
	"fmt"
	"go/ast"
	"strings"
)

// exprText prints an expression for a reason, in the form types.ExprString
// gives it, with one difference: a composite literal is written out, elements
// and all, where ExprString abbreviates it to T{…}. Two paths that differ only
// inside a literal, d.Plans[len([...]int{1})] and d.Plans[len([...]int{1, 2})],
// then read apart in a reason that names both. The comparisons a rule makes by
// spelling (N1, P6, V6, V7) still read ExprString; only what a reason shows
// changes. A function literal prints as (func() literal), as in ExprString: S9
// refuses it before any reason names it.
func exprText(e ast.Expr) string {
	var b strings.Builder
	writeExpr(&b, e)
	return b.String()
}

func writeExpr(b *strings.Builder, e ast.Expr) {
	switch x := e.(type) {
	default:
		fmt.Fprintf(b, "(ast: %T)", x) // nil or a BadExpr
	case *ast.Ident:
		b.WriteString(x.Name)
	case *ast.BasicLit:
		b.WriteString(x.Value)
	case *ast.Ellipsis:
		b.WriteString("...")
		if x.Elt != nil {
			writeExpr(b, x.Elt)
		}
	case *ast.FuncLit:
		b.WriteByte('(')
		writeExpr(b, x.Type)
		b.WriteString(" literal)")
	case *ast.CompositeLit:
		// The type is elided in an element of a literal of literals:
		// [][]int{{1}} is printed as written.
		if x.Type != nil {
			writeExpr(b, x.Type)
		}
		b.WriteByte('{')
		writeExprList(b, x.Elts)
		b.WriteByte('}')
	case *ast.KeyValueExpr:
		writeExpr(b, x.Key)
		b.WriteString(": ")
		writeExpr(b, x.Value)
	case *ast.ParenExpr:
		b.WriteByte('(')
		writeExpr(b, x.X)
		b.WriteByte(')')
	case *ast.SelectorExpr:
		writeExpr(b, x.X)
		b.WriteByte('.')
		b.WriteString(x.Sel.Name)
	case *ast.IndexExpr:
		writeExpr(b, x.X)
		b.WriteByte('[')
		writeExpr(b, x.Index)
		b.WriteByte(']')
	case *ast.IndexListExpr:
		writeExpr(b, x.X)
		b.WriteByte('[')
		writeExprList(b, x.Indices)
		b.WriteByte(']')
	case *ast.SliceExpr:
		writeExpr(b, x.X)
		b.WriteByte('[')
		if x.Low != nil {
			writeExpr(b, x.Low)
		}
		b.WriteByte(':')
		if x.High != nil {
			writeExpr(b, x.High)
		}
		if x.Slice3 {
			b.WriteByte(':')
			if x.Max != nil {
				writeExpr(b, x.Max)
			}
		}
		b.WriteByte(']')
	case *ast.TypeAssertExpr:
		writeExpr(b, x.X)
		b.WriteString(".(")
		if x.Type == nil {
			b.WriteString("type")
		} else {
			writeExpr(b, x.Type)
		}
		b.WriteByte(')')
	case *ast.CallExpr:
		writeExpr(b, x.Fun)
		b.WriteByte('(')
		writeExprList(b, x.Args)
		if x.Ellipsis.IsValid() {
			b.WriteString("...")
		}
		b.WriteByte(')')
	case *ast.StarExpr:
		b.WriteByte('*')
		writeExpr(b, x.X)
	case *ast.UnaryExpr:
		b.WriteString(x.Op.String())
		writeExpr(b, x.X)
	case *ast.BinaryExpr:
		writeExpr(b, x.X)
		b.WriteByte(' ')
		b.WriteString(x.Op.String())
		b.WriteByte(' ')
		writeExpr(b, x.Y)
	case *ast.ArrayType:
		b.WriteByte('[')
		if x.Len != nil {
			writeExpr(b, x.Len)
		}
		b.WriteByte(']')
		writeExpr(b, x.Elt)
	case *ast.StructType:
		b.WriteString("struct{")
		writeFields(b, x.Fields.List, "; ", false)
		b.WriteByte('}')
	case *ast.FuncType:
		b.WriteString("func")
		writeSignature(b, x)
	case *ast.InterfaceType:
		b.WriteString("interface{")
		writeFields(b, x.Methods.List, "; ", true)
		b.WriteByte('}')
	case *ast.MapType:
		b.WriteString("map[")
		writeExpr(b, x.Key)
		b.WriteByte(']')
		writeExpr(b, x.Value)
	case *ast.ChanType:
		switch x.Dir {
		case ast.SEND:
			b.WriteString("chan<- ")
		case ast.RECV:
			b.WriteString("<-chan ")
		default:
			b.WriteString("chan ")
		}
		writeExpr(b, x.Value)
	}
}

// writeSignature prints the parameters and results of a function type: no
// result, one unnamed result bare, several or named ones in parentheses.
func writeSignature(b *strings.Builder, sig *ast.FuncType) {
	b.WriteByte('(')
	writeFields(b, sig.Params.List, ", ", false)
	b.WriteByte(')')
	res := sig.Results
	switch n := res.NumFields(); {
	case n == 0:
	case n == 1 && len(res.List[0].Names) == 0:
		b.WriteByte(' ')
		writeExpr(b, res.List[0].Type)
	default:
		b.WriteString(" (")
		writeFields(b, res.List, ", ", false)
		b.WriteByte(')')
	}
}

// writeFields prints a field list: names, then the type, with a blank
// between them when there are names. An interface method is its name and
// signature. Tags are not printed.
func writeFields(b *strings.Builder, fields []*ast.Field, sep string, iface bool) {
	for i, f := range fields {
		if i > 0 {
			b.WriteString(sep)
		}
		for j, name := range f.Names {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name.Name)
		}
		if sig, ok := f.Type.(*ast.FuncType); ok && iface {
			writeSignature(b, sig)
			continue
		}
		if len(f.Names) > 0 {
			b.WriteByte(' ')
		}
		writeExpr(b, f.Type)
	}
}

func writeExprList(b *strings.Builder, list []ast.Expr) {
	for i, e := range list {
		if i > 0 {
			b.WriteString(", ")
		}
		writeExpr(b, e)
	}
}
