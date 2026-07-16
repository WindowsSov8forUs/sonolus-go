package frontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"golang.org/x/tools/go/packages"
)

func validStreamInitializer(pkg *packages.Package, named *types.Named, expression ast.Expr) bool {
	for {
		switch value := expression.(type) {
		case *ast.ParenExpr:
			expression = value.X
			continue
		case *ast.UnaryExpr:
			if value.Op != token.AND {
				return false
			}
			expression = value.X
			continue
		case *ast.CompositeLit:
			return len(value.Elts) == 0 && types.Identical(pkg.TypesInfo.TypeOf(value), named)
		default:
			return false
		}
	}
}

func streamElementSlots(t types.Type) (int, int, string, bool, error) {
	t = types.Unalias(t)
	if array, ok := t.Underlying().(*types.Array); ok {
		valueSlots, size, kind, stream, err := streamElementSlots(array.Elem())
		return valueSlots, int(array.Len()) * size, kind, stream, err
	}
	named, ok := namedType(t)
	if !ok || (typeID(named) != rootID("Stream") && typeID(named) != rootID("StreamData")) {
		return 0, 0, "", false, nil
	}
	if named.TypeArgs().Len() != 1 {
		return 0, 0, "", true, fmt.Errorf("stream type requires exactly one type argument")
	}
	valueSlots, err := layoutSize(named.TypeArgs().At(0))
	if err != nil {
		return 0, 0, "", true, err
	}
	kind := "stream"
	size := valueSlots
	if typeID(named) == rootID("StreamData") {
		kind, size = "data", 1
	}
	if size == 0 {
		size = 1
	}
	return valueSlots, size, kind, true, nil
}

func parseStreams(named *types.Named, variable *types.Var) (*StreamDeclaration, map[*types.Var][]int, []error) {
	result := &StreamDeclaration{PackagePath: named.Obj().Pkg().Path(), TypeName: named.Obj().Name(), Variable: variable.Name()}
	ids := map[*types.Var][]int{}
	var errs []error
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return result, ids, []error{fmt.Errorf("%s: stream resource must be a struct", named.Obj().Name())}
	}
	offset := 1
	for index := 0; index < st.NumFields(); index++ {
		field := st.Field(index)
		if field.Embedded() && typeID(field.Type()) == rootID("StreamResource") {
			continue
		}
		if field.Embedded() {
			errs = append(errs, fmt.Errorf("%s.%s: stream resource does not allow embedded fields", named.Obj().Name(), field.Name()))
			continue
		}
		valueSlots, size, kind, stream, err := streamElementSlots(field.Type())
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.%s: %w", named.Obj().Name(), field.Name(), err))
			continue
		}
		if !stream || size <= 0 {
			errs = append(errs, fmt.Errorf("%s.%s: stream resource fields must be Stream, StreamData, or fixed arrays of them", named.Obj().Name(), field.Name()))
			continue
		}
		fieldIDs := make([]int, size)
		for i := range fieldIDs {
			fieldIDs[i] = offset + i
		}
		ids[field] = fieldIDs
		result.Fields = append(result.Fields, StreamFieldDeclaration{Name: field.Name(), Type: types.TypeString(field.Type(), nil), Kind: kind, ValueSlots: valueSlots, Size: size})
		offset += size
	}
	result.Size = offset
	return result, ids, errs
}

func compareStreams(left, right *StreamDeclaration) error {
	if reflect.DeepEqual(left, right) {
		return nil
	}
	if left == nil || right == nil {
		return fmt.Errorf("stream resource must be declared identically in play and watch modes")
	}
	if len(left.Fields) != len(right.Fields) {
		return fmt.Errorf("stream resource field count differs between play (%d) and watch (%d)", len(left.Fields), len(right.Fields))
	}
	for index := range left.Fields {
		playField, watchField := left.Fields[index], right.Fields[index]
		if reflect.DeepEqual(playField, watchField) {
			continue
		}
		path := playField.Name
		if path == "" {
			path = watchField.Name
		}
		return fmt.Errorf("stream resource field %s differs between play (%s, kind=%s, slots=%d, size=%d) and watch (%s, kind=%s, slots=%d, size=%d)",
			path, playField.Type, playField.Kind, playField.ValueSlots, playField.Size,
			watchField.Type, watchField.Kind, watchField.ValueSlots, watchField.Size)
	}
	return nil
}
