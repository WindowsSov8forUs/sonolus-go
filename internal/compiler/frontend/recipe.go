package frontend

import "github.com/WindowsSov8forUs/sonolus-go/internal/compiler/catalog"

func operationSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

var aggregateOperations = operationSet(
	"ease", "control.terminate", "touch.get", "time.previous", "time.offsetAdjusted", "screen.rect",
	"vec2.new", "vec2.add", "vec2.sub", "vec2.mul", "vec2.div", "vec2.dot", "vec2.magnitudeSquared",
	"vec2.magnitude", "vec2.angle", "vec2.orthogonal", "vec2.normalize", "vec2.normalizeOrZero", "vec2.rotate",
	"vec2.rotateAbout", "vec2.angleDiff", "vec2.signedAngleDiff",
	"rect.width", "rect.height", "rect.center", "rect.translate", "rect.scale", "rect.toQuad", "rect.contains",
	"quad.center", "quad.translate", "quad.scale", "quad.rotate", "quad.top", "quad.right", "quad.bottom",
	"quad.left", "quad.permute", "quad.contains",
	"transform.translate", "transform.scale", "transform.rotate", "transform.compose", "transform.composeBefore",
	"transform.scaleAbout", "transform.rotateAbout", "transform.vec", "transform.quad",
)

var resourceOperations = operationSet(
	"archetype.spawn", "sprite.draw", "sprite.exists", "sprite.drawCurvedB", "sprite.drawCurvedT",
	"sprite.drawCurvedL", "sprite.drawCurvedR", "sprite.drawCurvedBT", "sprite.drawCurvedLR", "judgment.judge",
	"clip.play", "clip.playScheduled", "clip.playLooped", "clip.playLoopedScheduled", "loop.stop",
	"loop.stopScheduled", "particle.spawn", "particle.move", "particle.destroy", "instruction.show", "instruction.clear",
)

var containerOperations = operationSet(
	"varArray.new", "varArray.len", "varArray.capacity", "varArray.isFull", "varArray.get", "varArray.set",
	"varArray.append", "varArray.pop", "varArray.clear", "varArray.contains", "varArray.insert",
	"arrayMap.new", "arrayMap.len", "arrayMap.capacity", "arrayMap.get", "arrayMap.set", "arrayMap.delete",
	"arrayMap.contains", "arrayMap.clear",
	"arraySet.new", "arraySet.len", "arraySet.capacity", "arraySet.add", "arraySet.remove", "arraySet.contains", "arraySet.clear",
)

func supportsRecipe(recipe catalog.Recipe) bool {
	var operations map[string]struct{}
	switch recipe.Kind {
	case catalog.RecipeAggregate:
		operations = aggregateOperations
	case catalog.RecipeResource:
		operations = resourceOperations
	case catalog.RecipeContainer:
		operations = containerOperations
	default:
		return true
	}
	_, ok := operations[recipe.Operation]
	return ok
}
