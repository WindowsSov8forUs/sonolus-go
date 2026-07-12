package tag

import (
	"go/ast"
	"go/token"
	"reflect"
	"testing"
)

func TestParseAndUnknown(t *testing.T) {
	value, ok := Parse(`json:"x" sonolus:"memory, name = hit ,unknown=true,flag"`, "sonolus")
	if !ok || !value.Flags["memory"] || !value.Flags["flag"] || value.Items["name"] != "hit" {
		t.Fatalf("value = %#v", value)
	}
	if got := value.Unknown([]string{"memory"}, []string{"name"}); !reflect.DeepEqual(got, []string{"flag", "unknown"}) {
		t.Fatalf("unknown = %#v", got)
	}
}

func TestParseLiteralAndMissing(t *testing.T) {
	literal := &ast.BasicLit{Kind: token.STRING, Value: "`configuration:\"toggle,def=true\"`"}
	value, ok := ParseLiteral(literal, "configuration")
	if !ok || !value.Flags["toggle"] || value.Items["def"] != "true" {
		t.Fatalf("value = %#v", value)
	}
	if _, ok := ParseLiteral(literal, "sonolus"); ok {
		t.Fatal("unexpected sonolus tag")
	}
}
