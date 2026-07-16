package frontend

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/packages"

	"github.com/WindowsSov8forUs/sonolus-core-go/core"
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

func resourceSpriteID(out *ModeDeclarations, pkg *packages.Package, expr ast.Expr) (int, bool) {
	if index, ok := expr.(*ast.IndexExpr); ok {
		selector, selectorOK := index.X.(*ast.SelectorExpr)
		if !selectorOK {
			return 0, false
		}
		field, fieldOK := pkg.TypesInfo.ObjectOf(selector.Sel).(*types.Var)
		value := pkg.TypesInfo.Types[index.Index].Value
		position, positionOK := constant.Int64Val(value)
		ids := out.Resources.FieldIDs[field]
		if !fieldOK || !positionOK || position < 0 || position >= int64(len(ids)) {
			return 0, false
		}
		return ids[position], true
	}
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return 0, false
	}
	field, ok := pkg.TypesInfo.ObjectOf(selector.Sel).(*types.Var)
	if !ok {
		return 0, false
	}
	ids := out.Resources.FieldIDs[field]
	if len(ids) != 1 {
		return 0, false
	}
	return ids[0], true
}

func bucketSprite(out *ModeDeclarations, pkg *packages.Package, expr ast.Expr) (resource.EngineDataBucketSprite, error) {
	call, plain := calledFunction(pkg, expr, "JudgmentBucketSprite")
	if !plain {
		call, plain = calledFunction(pkg, expr, "JudgmentBucketSpriteWithFallback")
	}
	want := 6
	if !plain {
		return resource.EngineDataBucketSprite{}, fmt.Errorf("use sonolus.JudgmentBucketSprite or sonolus.JudgmentBucketSpriteWithFallback")
	}
	withFallback := false
	if objectCall, ok := calledFunction(pkg, expr, "JudgmentBucketSpriteWithFallback"); ok {
		call = objectCall
		want = 7
		withFallback = true
	}
	if len(call.Args) != want {
		return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket sprite constructor has %d arguments; want %d", len(call.Args), want)
	}
	id, ok := resourceSpriteID(out, pkg, call.Args[0])
	if !ok {
		return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket sprite must reference a declared skin sprite field")
	}
	result := resource.EngineDataBucketSprite{ID: id}
	start := 1
	if withFallback {
		fallback, valid := resourceSpriteID(out, pkg, call.Args[1])
		if !valid {
			return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket fallback must reference a declared skin sprite field")
		}
		result.FallbackID = fallback
		start = 2
	}
	values := []*float64{&result.X, &result.Y, &result.W, &result.H, &result.Rotation}
	for index, target := range values {
		value, valid := constantFloat(pkg, call.Args[start+index])
		if !valid {
			return resource.EngineDataBucketSprite{}, fmt.Errorf("bucket sprite geometry must use constant numbers")
		}
		*target = value
	}
	return result, nil
}

func addBucketResource(out *ModeDeclarations, spec resourceDeclarationSpec) []error {
	literal, ok := unwrapResourceLiteral(spec.initializer)
	if !ok {
		return []error{fmt.Errorf("%s: buckets resource value must be a struct literal or pointer to one", spec.pos)}
	}
	st := spec.named.Underlying().(*types.Struct)
	values := map[string]ast.Expr{}
	position := 0
	for _, element := range literal.Elts {
		if pair, keyed := element.(*ast.KeyValueExpr); keyed {
			if name, valid := pair.Key.(*ast.Ident); valid {
				values[name.Name] = pair.Value
			}
			continue
		}
		if position < st.NumFields() {
			values[st.Field(position).Name()] = element
			position++
		}
	}
	var errs []error
	for index := 0; index < st.NumFields(); index++ {
		field := st.Field(index)
		if field == spec.marker {
			continue
		}
		if field.Embedded() || typeID(field.Type()) != rootID("Bucket") {
			errs = append(errs, fmt.Errorf("%s.%s: buckets resource fields must be sonolus.Bucket", spec.named.Obj().Name(), field.Name()))
			continue
		}
		call, valid := calledFunction(spec.pkg, values[field.Name()], "JudgmentBucket")
		if !valid || len(call.Args) < 1 {
			errs = append(errs, fmt.Errorf("%s.%s: use sonolus.JudgmentBucket(unit, sprites...)", spec.named.Obj().Name(), field.Name()))
			continue
		}
		unitValue := spec.pkg.TypesInfo.Types[call.Args[0]].Value
		if unitValue == nil || unitValue.Kind() != constant.String {
			errs = append(errs, fmt.Errorf("%s.%s: bucket unit must be a constant string", spec.named.Obj().Name(), field.Name()))
			continue
		}
		bucket := resource.EngineDataBucket{Unit: core.Text(constant.StringVal(unitValue))}
		for _, expr := range call.Args[1:] {
			sprite, err := bucketSprite(out, spec.pkg, expr)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: %w", spec.named.Obj().Name(), field.Name(), err))
				continue
			}
			bucket.Sprites = append(bucket.Sprites, sprite)
		}
		out.Resources.FieldIDs[field] = []int{len(out.Resources.Buckets)}
		out.Resources.Buckets = append(out.Resources.Buckets, bucket)
	}
	return errs
}
